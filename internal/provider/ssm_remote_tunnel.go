package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &SSMRemoteTunnelDataSource{}

func NewSSMRemoteTunnelDataSource() datasource.DataSource {
	return &SSMRemoteTunnelDataSource{}
}

// SSMRemoteTunnelDataSource defines the data source implementation.
type SSMRemoteTunnelDataSource struct {
	tracker *TunnelTracker
}

// SSMRemoteTunnelDataSourceModel describes the data source data model.
type SSMRemoteTunnelDataSourceModel struct {
	Target     types.String `tfsdk:"target"`
	RemoteHost types.String `tfsdk:"remote_host"`
	RemotePort types.Int64  `tfsdk:"remote_port"`
	LocalPort  types.Int64  `tfsdk:"local_port"`
	Region     types.String `tfsdk:"region"`
	Id         types.String `tfsdk:"id"`
}

func (d *SSMRemoteTunnelDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ssm_remote_tunnel"
}

func (d *SSMRemoteTunnelDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "AWSM SSM Remote Tunnel data source",

		Attributes: map[string]schema.Attribute{
			"target": schema.StringAttribute{
				MarkdownDescription: "The target to start the remote tunnel, such as an instance ID",
				Required:            true,
			},
			"remote_host": schema.StringAttribute{
				MarkdownDescription: "The DNS name or IP address of the remote host",
				Required:            true,
			},
			"remote_port": schema.Int64Attribute{
				MarkdownDescription: "The port number of the remote host",
				Required:            true,
			},
			"local_port": schema.Int64Attribute{
				MarkdownDescription: "The local port number to use for the tunnel",
				Required:            true,
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "The AWS region to use for the tunnel. This should match the region of the target",
				Required:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Example identifier", // TODO: Figure this out
				Computed:            true,
			},
		},
	}
}

func (d *SSMRemoteTunnelDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	tracker, ok := req.ProviderData.(*TunnelTracker)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *TunnelTracker, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.tracker = tracker
}

func (d *SSMRemoteTunnelDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SSMRemoteTunnelDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := d.tracker.StartTunnel(
		ctx,
		data.Id.ValueString(),
		data.Target.ValueString(),
		data.RemoteHost.ValueString(),
		data.RemotePort.ValueInt64(),
		data.LocalPort.ValueInt64(),
		data.Region.ValueString(),
	)

	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to start remote tunnel",
			fmt.Sprintf("Error: %s", err),
		)
	}

	// Wait for the tunnel to be ready
	tunnelInfo, exists := d.tracker.Tunnels[data.Id.ValueString()]
	if !exists {
		resp.Diagnostics.AddError(
			"Tunnel not found",
			"The requested tunnel does not exist in the tracker.",
		)
		return
	}

	// This blocks until the tunnel is ready or the context is done
	select {
	case <-tunnelInfo.ReadySignal:
		// Tunnel is ready. Proceed.
		// time.Sleep(10 * time.Second)
		// Save data into Terraform state
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	case <-ctx.Done():
		// Context was cancelled or timed out. Handle accordingly.
		resp.Diagnostics.AddError(
			"Context cancelled or timed out",
			"The operation was cancelled or timed out before the tunnel became ready.",
		)
		return
	}
}
