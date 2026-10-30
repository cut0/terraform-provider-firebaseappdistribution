package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ provider.Provider = &appDistributionProvider{}

type appDistributionProvider struct {
	version string
}

type appDistributionProviderModel struct {
	Project                   types.String `tfsdk:"project"`
	AccessToken               types.String `tfsdk:"access_token"`
	ImpersonateServiceAccount types.String `tfsdk:"impersonate_service_account"`
	BillingProject            types.String `tfsdk:"billing_project"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &appDistributionProvider{version: version}
	}
}

func (p *appDistributionProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "firebaseappdistribution"
	resp.Version = p.version
}

func (p *appDistributionProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage [Firebase App Distribution](https://firebase.google.com/docs/app-distribution) " +
			"tester groups and group memberships through the " +
			"[App Distribution REST API](https://firebase.google.com/docs/reference/app-distribution/rest).\n\n" +
			"Authentication follows the usual Google Cloud conventions. Unless `access_token` is set, " +
			"[Application Default Credentials](https://cloud.google.com/docs/authentication/application-default-credentials) " +
			"are used, which covers `gcloud auth application-default login`, the `GOOGLE_APPLICATION_CREDENTIALS` " +
			"environment variable and Workload Identity Federation in CI.\n\n" +
			"Projects can be referenced by ID or by number. IDs are resolved to numbers through the " +
			"[Firebase Management API](https://firebase.google.com/docs/reference/firebase-management/rest), which " +
			"must be enabled on the project. The identity needs `firebase.projects.get` plus the " +
			"`firebaseappdistro.groups.*` and `firebaseappdistro.testers.*` permissions; the predefined roles " +
			"`roles/firebase.viewer` and `roles/firebaseappdistro.admin` together grant them.",
		Attributes: map[string]schema.Attribute{
			"project": schema.StringAttribute{
				MarkdownDescription: "Default Firebase project ID or number used by resources that do not set `project`. " +
					"Can also be set with the `FIREBASE_PROJECT` or `GOOGLE_PROJECT` environment variable.",
				Optional: true,
			},
			"access_token": schema.StringAttribute{
				MarkdownDescription: "A temporary OAuth 2.0 access token obtained from the Google Authorization server. " +
					"Can also be set with the `GOOGLE_OAUTH_ACCESS_TOKEN` environment variable. " +
					"When set, Application Default Credentials are not used.",
				Optional:  true,
				Sensitive: true,
			},
			"impersonate_service_account": schema.StringAttribute{
				MarkdownDescription: "Email of a service account to impersonate for all API calls. " +
					"The base identity needs `roles/iam.serviceAccountTokenCreator` on it. " +
					"Can also be set with the `GOOGLE_IMPERSONATE_SERVICE_ACCOUNT` environment variable.",
				Optional: true,
			},
			"billing_project": schema.StringAttribute{
				MarkdownDescription: "Project used for quota and billing (sent as `x-goog-user-project`). " +
					"Needed when the identity belongs to a different project than the one being managed. " +
					"Can also be set with the `GOOGLE_BILLING_PROJECT` environment variable.",
				Optional: true,
			},
		},
	}
}

func (p *appDistributionProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config appDistributionProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	unknownAttributes := map[string]types.String{
		"project":                     config.Project,
		"access_token":                config.AccessToken,
		"impersonate_service_account": config.ImpersonateServiceAccount,
		"billing_project":             config.BillingProject,
	}
	for name, value := range unknownAttributes {
		if value.IsUnknown() {
			resp.Diagnostics.AddAttributeError(
				path.Root(name),
				"Unknown provider configuration value",
				"The provider cannot be configured because `"+name+"` is not known until apply. "+
					"Set it to a literal value or an environment variable, or ensure the value is known at plan time.",
			)
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}

	project := firstNonEmpty(config.Project.ValueString(), os.Getenv("FIREBASE_PROJECT"), os.Getenv("GOOGLE_PROJECT"))
	accessToken := firstNonEmpty(config.AccessToken.ValueString(), os.Getenv("GOOGLE_OAUTH_ACCESS_TOKEN"))
	impersonate := firstNonEmpty(config.ImpersonateServiceAccount.ValueString(), os.Getenv("GOOGLE_IMPERSONATE_SERVICE_ACCOUNT"))
	billingProject := firstNonEmpty(config.BillingProject.ValueString(), os.Getenv("GOOGLE_BILLING_PROJECT"))

	apiClient, err := newClient(ctx, clientConfig{
		project:        project,
		accessToken:    accessToken,
		impersonate:    impersonate,
		billingProject: billingProject,
		userAgent:      "terraform-provider-firebaseappdistribution/" + p.version,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create Firebase API clients",
			"An unexpected error occurred while configuring Google credentials: "+err.Error(),
		)
		return
	}

	tflog.Debug(ctx, "configured Firebase API clients", map[string]any{"project": project})

	resp.DataSourceData = apiClient
	resp.ResourceData = apiClient
}

func (p *appDistributionProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewGroupResource,
		NewGroupMemberResource,
	}
}

func (p *appDistributionProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewGroupDataSource,
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
