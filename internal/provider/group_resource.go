package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/api/firebaseappdistribution/v1"
)

var (
	_ resource.Resource                = &groupResource{}
	_ resource.ResourceWithConfigure   = &groupResource{}
	_ resource.ResourceWithImportState = &groupResource{}
)

type groupResource struct {
	client *client
}

type groupResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Project         types.String `tfsdk:"project"`
	GroupID         types.String `tfsdk:"group_id"`
	DisplayName     types.String `tfsdk:"display_name"`
	Name            types.String `tfsdk:"name"`
	TesterCount     types.Int64  `tfsdk:"tester_count"`
	ReleaseCount    types.Int64  `tfsdk:"release_count"`
	InviteLinkCount types.Int64  `tfsdk:"invite_link_count"`
}

func NewGroupResource() resource.Resource {
	return &groupResource{}
}

func (r *groupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (r *groupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A tester group in Firebase App Distribution. Groups are scoped to a Firebase project, " +
			"not to an individual app; which app releases are distributed to a group is decided when a release is " +
			"distributed. Use `firebaseappdistribution_group_member` to manage the testers in the group.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Full resource name of the group, in the form `projects/{project_number}/groups/{group_id}`.",
				Computed:            true,
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
				MarkdownDescription: "Alias of the group, which becomes the last segment of its resource name and is what " +
					"`releases:distribute` and the Firebase CLI refer to as the group alias. Changing this forces a new resource.",
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					stringvalidator.RegexMatches(regexp.MustCompile(`^[^/]+$`), "must not contain '/'"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Human readable name shown in the Firebase console.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Full resource name of the group as returned by the API. Same as `id`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tester_count": schema.Int64Attribute{
				MarkdownDescription: "Number of testers in the group.",
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"release_count": schema.Int64Attribute{
				MarkdownDescription: "Number of releases the group has access to.",
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"invite_link_count": schema.Int64Attribute{
				MarkdownDescription: "Number of invite links for the group. Invite links are not exposed by the API and " +
					"must be managed in the Firebase console.",
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *groupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *groupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupResourceModel

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

	group, err := r.client.appDistribution.Projects.Groups.Create(projectParent(projectNumber), &firebaseappdistribution.GoogleFirebaseAppdistroV1Group{
		DisplayName: plan.DisplayName.ValueString(),
	}).GroupId(plan.GroupID.ValueString()).Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError("Error creating group", fmt.Sprintf("Unable to create group %q: %s", plan.GroupID.ValueString(), err))
		return
	}

	tflog.Trace(ctx, "created App Distribution group", map[string]any{"name": group.Name})

	plan.Project = types.StringValue(project)
	plan.fromAPI(group)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *groupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state groupResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name, err := r.resourceName(ctx, state)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving project", err.Error())
		return
	}

	group, err := r.client.appDistribution.Projects.Groups.Get(name).Context(ctx).Do()
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading group", fmt.Sprintf("Unable to read group %q: %s", name, err))
		return
	}

	state.fromAPI(group)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *groupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state groupResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name, err := r.resourceName(ctx, state)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving project", err.Error())
		return
	}

	group, err := r.client.appDistribution.Projects.Groups.Patch(name, &firebaseappdistribution.GoogleFirebaseAppdistroV1Group{
		DisplayName: plan.DisplayName.ValueString(),
	}).UpdateMask("displayName").Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError("Error updating group", fmt.Sprintf("Unable to update group %q: %s", name, err))
		return
	}

	plan.Project = state.Project
	plan.fromAPI(group)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *groupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state groupResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name, err := r.resourceName(ctx, state)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving project", err.Error())
		return
	}

	_, err = r.client.appDistribution.Projects.Groups.Delete(name).Context(ctx).Do()
	if err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("Error deleting group", fmt.Sprintf("Unable to delete group %q: %s", name, err))
		return
	}
}

func (r *groupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	project, groupID, ok := parseGroupName(req.ID)
	if !ok {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected `projects/{project}/groups/{group_id}`, got %q.", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), project)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("group_id"), groupID)...)
}

func (r *groupResource) resourceName(ctx context.Context, state groupResourceModel) (string, error) {
	projectNumber, err := r.client.projectNumber(ctx, state.Project.ValueString())
	if err != nil {
		return "", err
	}
	return groupName(projectNumber, state.GroupID.ValueString()), nil
}

func (m *groupResourceModel) fromAPI(group *firebaseappdistribution.GoogleFirebaseAppdistroV1Group) {
	m.ID = types.StringValue(group.Name)
	m.Name = types.StringValue(group.Name)
	m.DisplayName = types.StringValue(group.DisplayName)
	m.TesterCount = types.Int64Value(group.TesterCount)
	m.ReleaseCount = types.Int64Value(group.ReleaseCount)
	m.InviteLinkCount = types.Int64Value(group.InviteLinkCount)

	if _, groupID, ok := parseGroupName(group.Name); ok {
		m.GroupID = types.StringValue(groupID)
	}
}
