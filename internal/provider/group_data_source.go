package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = &groupDataSource{}
	_ datasource.DataSourceWithConfigure = &groupDataSource{}
)

type groupDataSource struct {
	client *client
}

type groupDataSourceModel struct {
	ID              types.String `tfsdk:"id"`
	Project         types.String `tfsdk:"project"`
	GroupID         types.String `tfsdk:"group_id"`
	DisplayName     types.String `tfsdk:"display_name"`
	Name            types.String `tfsdk:"name"`
	TesterCount     types.Int64  `tfsdk:"tester_count"`
	ReleaseCount    types.Int64  `tfsdk:"release_count"`
	InviteLinkCount types.Int64  `tfsdk:"invite_link_count"`
}

func NewGroupDataSource() datasource.DataSource {
	return &groupDataSource{}
}

func (d *groupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (d *groupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads an existing Firebase App Distribution tester group, for example one created in the " +
			"Firebase console, so that its alias can be referenced without importing it.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Full resource name of the group, in the form `projects/{project_number}/groups/{group_id}`.",
				Computed:            true,
			},
			"project": schema.StringAttribute{
				MarkdownDescription: "Firebase project ID or number. Defaults to the provider `project`.",
				Optional:            true,
				Computed:            true,
			},
			"group_id": schema.StringAttribute{
				MarkdownDescription: "Alias of the group.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					stringvalidator.RegexMatches(regexp.MustCompile(`^[^/]+$`), "must not contain '/'"),
				},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Human readable name shown in the Firebase console.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Full resource name of the group as returned by the API. Same as `id`.",
				Computed:            true,
			},
			"tester_count": schema.Int64Attribute{
				MarkdownDescription: "Number of testers in the group.",
				Computed:            true,
			},
			"release_count": schema.Int64Attribute{
				MarkdownDescription: "Number of releases the group has access to.",
				Computed:            true,
			},
			"invite_link_count": schema.Int64Attribute{
				MarkdownDescription: "Number of invite links for the group.",
				Computed:            true,
			},
		},
	}
}

func (d *groupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	apiClient, ok := req.ProviderData.(*client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = apiClient
}

func (d *groupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config groupDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	project := d.client.projectOrDefault(config.Project.ValueString())
	if project == "" {
		resp.Diagnostics.AddAttributeError(path.Root("project"), "Missing project", "Set `project` on the data source or the provider.")
		return
	}

	projectNumber, err := d.client.projectNumber(ctx, project)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving project", err.Error())
		return
	}

	name := groupName(projectNumber, config.GroupID.ValueString())
	group, err := d.client.appDistribution.Projects.Groups.Get(name).Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError("Error reading group", fmt.Sprintf("Unable to read group %q: %s", name, err))
		return
	}

	config.ID = types.StringValue(group.Name)
	config.Name = types.StringValue(group.Name)
	config.Project = types.StringValue(project)
	config.DisplayName = types.StringValue(group.DisplayName)
	config.TesterCount = types.Int64Value(group.TesterCount)
	config.ReleaseCount = types.Int64Value(group.ReleaseCount)
	config.InviteLinkCount = types.Int64Value(group.InviteLinkCount)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
