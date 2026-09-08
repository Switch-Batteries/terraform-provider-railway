package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &ServiceRegistryCredentialsResource{}
var _ resource.ResourceWithImportState = &ServiceRegistryCredentialsResource{}

func NewServiceRegistryCredentialsResource() resource.Resource {
	return &ServiceRegistryCredentialsResource{}
}

type ServiceRegistryCredentialsResource struct {
	client *graphql.Client
}

type ServiceRegistryCredentialsResourceModel struct {
	Id            types.String `tfsdk:"id"`
	EnvironmentId types.String `tfsdk:"environment_id"`
	ServiceId     types.String `tfsdk:"service_id"`
	Username      types.String `tfsdk:"username"`
	Password      types.String `tfsdk:"password"`
}

func (r *ServiceRegistryCredentialsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_registry_credentials"
}

func (r *ServiceRegistryCredentialsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The credential a service instance pulls its image with, for a service Terraform does not otherwise manage. Railway never returns the credential, so this resource writes it on create and whenever the values change, and cannot detect a change made elsewhere.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the credential, `service_id:environment_id`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the environment the service instance belongs to.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(uuidRegex(), "must be an id"),
				},
			},
			"service_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the service.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(uuidRegex(), "must be an id"),
				},
			},
			"username": schema.StringAttribute{
				MarkdownDescription: "Registry username.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Registry password or token.",
				Required:            true,
				Sensitive:           true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
		},
	}
}

func (r *ServiceRegistryCredentialsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*graphql.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *graphql.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *ServiceRegistryCredentialsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *ServiceRegistryCredentialsResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.write(ctx, data, data.Username.ValueString(), data.Password.ValueString()); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to set registry credentials, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "set registry credentials on a service instance")

	data.Id = types.StringValue(credentialsId(data))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Railway does not return registry credentials, so state is the record.
func (r *ServiceRegistryCredentialsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *ServiceRegistryCredentialsResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ServiceRegistryCredentialsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *ServiceRegistryCredentialsResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.write(ctx, data, data.Username.ValueString(), data.Password.ValueString()); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update registry credentials, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "updated registry credentials on a service instance")

	data.Id = types.StringValue(credentialsId(data))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Railway has no call that removes a credential; empty values are the nearest thing.
func (r *ServiceRegistryCredentialsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *ServiceRegistryCredentialsResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.write(ctx, data, "", ""); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to clear registry credentials, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "cleared registry credentials on a service instance")
}

func (r *ServiceRegistryCredentialsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, ":")

	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: service_id:environment_id. Got: %q", req.ID),
		)

		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("service_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), parts[1])...)
}

func (r *ServiceRegistryCredentialsResource) write(ctx context.Context, data *ServiceRegistryCredentialsResourceModel, username string, password string) error {
	input := ServiceInstanceUpdateInput{
		RegistryCredentials: &RegistryCredentialsInput{
			Username: username,
			Password: password,
		},
	}

	_, err := updateServiceInstanceRegistryCredentials(ctx, *r.client, data.EnvironmentId.ValueString(), data.ServiceId.ValueString(), input)

	return err
}

func credentialsId(data *ServiceRegistryCredentialsResourceModel) string {
	return fmt.Sprintf("%s:%s", data.ServiceId.ValueString(), data.EnvironmentId.ValueString())
}
