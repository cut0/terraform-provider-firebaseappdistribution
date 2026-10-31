package provider

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/api/firebaseappdistribution/v1"
)

var (
	_ resource.Resource                = &groupMemberResource{}
	_ resource.ResourceWithConfigure   = &groupMemberResource{}
	_ resource.ResourceWithImportState = &groupMemberResource{}
)

const (
	membershipReadAttempts = 6
	membershipReadInterval = 2 * time.Second
)

var errStopPaging = errors.New("stop paging")

type groupMemberResource struct {
	client *client
}

type groupMemberResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Project             types.String `tfsdk:"project"`
	GroupID             types.String `tfsdk:"group_id"`
	Email               types.String `tfsdk:"email"`
	CreateMissingTester types.Bool   `tfsdk:"create_missing_tester"`
	TesterName          types.String `tfsdk:"tester_name"`
	TesterDisplayName   types.String `tfsdk:"tester_display_name"`
}

func NewGroupMemberResource() resource.Resource {
	return &groupMemberResource{}
}

func (r *groupMemberResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_member"
}

func (r *groupMemberResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Membership of a single tester in a Firebase App Distribution group. " +
			"Creating the resource joins the tester to the group (`groups:batchJoin`) and destroying it removes the " +
			"tester from the group (`groups:batchLeave`). The tester itself is never removed from the project, so " +
			"destroying this resource does not revoke access to releases that were distributed to the tester directly.\n\n" +
			"Use one resource per tester, typically with `for_each` over a set of email addresses.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Synthetic ID in the form `projects/{project_number}/groups/{group_id}/testers/{email}`, " +
					"using the project number and the email as registered in the project.",
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project": schema.StringAttribute{
				MarkdownDescription: "Firebase project ID or number. Defaults to the provider `project`. Changing this forces a new resource.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"group_id": schema.StringAttribute{
				MarkdownDescription: "Alias of the group. Changing this forces a new resource.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					stringvalidator.RegexMatches(regexp.MustCompile(`^[^/]+$`), "must not contain '/'"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "Email address of the tester. Compared case-insensitively against the API. " +
					"Changing this forces a new resource.",
				Required: true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(`^[^\s/@]+@[^\s/@]+$`), "must be an email address"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"create_missing_tester": schema.BoolAttribute{
				MarkdownDescription: "Whether to register the email as a tester of the project when it is not one yet. " +
					"When `false`, joining an unknown email fails. Defaults to `true`.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"tester_name": schema.StringAttribute{
				MarkdownDescription: "Resource name of the tester, in the form `projects/{project_number}/testers/{email}`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tester_display_name": schema.StringAttribute{
				MarkdownDescription: "Display name of the tester as registered in the project.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *groupMemberResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	apiClient, ok := req.ProviderData.(*client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = apiClient
}

func (r *groupMemberResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupMemberResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	project := r.client.projectOrDefault(plan.Project.ValueString())
	if project == "" {
		resp.Diagnostics.AddAttributeError(path.Root("project"), "Missing project", "Set `project` on the resource or the provider.")
		return
	}

	projectNumber, err := r.client.projectNumber(ctx, project)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving project", err.Error())
		return
	}

	group := groupName(projectNumber, plan.GroupID.ValueString())
	email := plan.Email.ValueString()

	_, err = r.client.appDistribution.Projects.Groups.BatchJoin(group, &firebaseappdistribution.GoogleFirebaseAppdistroV1BatchJoinGroupRequest{
		Emails:               []string{email},
		CreateMissingTesters: plan.CreateMissingTester.ValueBool(),
	}).Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError("Error adding tester to group", fmt.Sprintf("Unable to add %q to %q: %s", email, group, err))
		return
	}

	tflog.Trace(ctx, "added tester to App Distribution group", map[string]any{"group": group})

	tester, err := r.waitForMembership(ctx, projectNumber, plan.GroupID.ValueString(), email)
	if err != nil {
		resp.Diagnostics.AddError("Error reading group membership", fmt.Sprintf("Unable to verify that %q joined %q: %s", email, group, err))
		return
	}
	if tester == nil {
		resp.Diagnostics.AddError(
			"Tester not found after join",
			fmt.Sprintf("%q was added to %q but the API does not list it as a member. Check that the email is registered as a tester of the project.", email, group),
		)
		return
	}

	plan.Project = types.StringValue(project)
	plan.fromAPI(projectNumber, tester)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *groupMemberResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state groupMemberResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectNumber, err := r.client.projectNumber(ctx, state.Project.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error resolving project", err.Error())
		return
	}

	tester, err := r.findMember(ctx, projectNumber, state.GroupID.ValueString(), state.Email.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading group membership", fmt.Sprintf("Unable to read membership %q: %s", state.ID.ValueString(), err))
		return
	}
	if tester == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.fromAPI(projectNumber, tester)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *groupMemberResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state groupMemberResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.CreateMissingTester = plan.CreateMissingTester

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *groupMemberResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state groupMemberResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectNumber, err := r.client.projectNumber(ctx, state.Project.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error resolving project", err.Error())
		return
	}

	group := groupName(projectNumber, state.GroupID.ValueString())

	_, err = r.client.appDistribution.Projects.Groups.BatchLeave(group, &firebaseappdistribution.GoogleFirebaseAppdistroV1BatchLeaveGroupRequest{
		Emails: []string{state.Email.ValueString()},
	}).Context(ctx).Do()
	if err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("Error removing tester from group", fmt.Sprintf("Unable to remove %q from %q: %s", state.Email.ValueString(), group, err))
		return
	}
}

func (r *groupMemberResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	project, groupID, email, ok := parseGroupMemberID(req.ID)
	if !ok {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected `projects/{project}/groups/{group_id}/testers/{email}`, got %q.", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), project)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("group_id"), groupID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("email"), email)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("create_missing_tester"), true)...)
}

func (r *groupMemberResource) waitForMembership(ctx context.Context, projectNumber, groupID, email string) (*firebaseappdistribution.GoogleFirebaseAppdistroV1Tester, error) {
	for attempt := 1; ; attempt++ {
		tester, err := r.findMember(ctx, projectNumber, groupID, email)
		if err != nil || tester != nil || attempt >= membershipReadAttempts {
			return tester, err
		}

		tflog.Debug(ctx, "membership not visible yet, retrying", map[string]any{"attempt": attempt})

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(membershipReadInterval):
		}
	}
}

func (r *groupMemberResource) findMember(ctx context.Context, projectNumber, groupID, email string) (*firebaseappdistribution.GoogleFirebaseAppdistroV1Tester, error) {
	var found *firebaseappdistribution.GoogleFirebaseAppdistroV1Tester

	filter := fmt.Sprintf(`groups = "%s"`, groupName(projectNumber, groupID))
	err := r.client.appDistribution.Projects.Testers.List(projectParent(projectNumber)).Filter(filter).PageSize(1000).Pages(ctx,
		func(page *firebaseappdistribution.GoogleFirebaseAppdistroV1ListTestersResponse) error {
			for _, tester := range page.Testers {
				if strings.EqualFold(testerEmail(tester.Name), email) {
					found = tester
					return errStopPaging
				}
			}
			return nil
		})
	if err != nil && !errors.Is(err, errStopPaging) {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return found, nil
}

func (m *groupMemberResourceModel) fromAPI(projectNumber string, tester *firebaseappdistribution.GoogleFirebaseAppdistroV1Tester) {
	m.ID = types.StringValue(groupMemberID(projectNumber, m.GroupID.ValueString(), testerEmail(tester.Name)))
	m.TesterName = types.StringValue(tester.Name)
	m.TesterDisplayName = types.StringValue(tester.DisplayName)
}
