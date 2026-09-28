package svgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sercanarga/pcileechgen/internal/firmware"
)

func TestSVPatcherAppliesBarMirror(t *testing.T) {
	dir := t.TempDir()
	writeBarMirrorFixture(t, dir)

	patcher := NewSVPatcher(firmware.DeviceIDs{}, dir)
	patcher.ShadowConfig = true
	patcher.MSIXCapDword = 0x2C

	if err := patcher.patchShadowConfigSpace(); err != nil {
		t.Fatalf("patchShadowConfigSpace: %v", err)
	}
	if err := patcher.patchBarMirror(); err != nil {
		t.Fatalf("patchBarMirror: %v", err)
	}
	if err := patcher.validateBarMirror(); err != nil {
		t.Fatalf("validateBarMirror: %v", err)
	}

	expected := map[string][]string{
		"pcileech_tlps128_cfgspace_shadow.sv": {
			"output reg  [9:0]       o_bar_wr_dwaddr",
			"output reg  [31:0]      o_bar_wr_data",
			"wire bar_wr = pcie_rx_wren & dshadow2fifo.cfgtlp_en &",
			"((pcie_rx_addr == 10'd4) | (pcie_rx_addr == 10'd5));",
			"o_bar_wr_dwaddr <= pcie_rx_addr;",
			"o_bar_wr_data   <= `_bs32(pcie_rx_data);",
		},
		"pcileech_pcie_tlp_a7.sv": {
			"output wire             bar_wr_valid",
			"output wire [9:0]       bar_wr_dwaddr",
			"output wire [31:0]      bar_wr_data",
			".o_bar_wr_valid ( bar_wr_valid                  ),",
			".o_bar_wr_dwaddr( bar_wr_dwaddr                 ),",
			".o_bar_wr_data  ( bar_wr_data                   )",
		},
		"pcileech_pcie_a7.sv": {
			"wire                    bar_wr_valid;",
			"wire [9:0]              bar_wr_dwaddr;",
			"wire [31:0]             bar_wr_data;",
			".bar_wr_dwaddr              ( bar_wr_dwaddr             ),",
		},
		"pcileech_pcie_cfg_a7.sv": {
			"input                   bar_wr_valid,",
			"input       [9:0]       bar_wr_dwaddr,",
			"input       [31:0]      bar_wr_data,",
			"reg     [1:0]       rwi_bar_cnt;",
			"reg     [9:0]       rwi_bar_dwaddr [0:1];",
			"reg     [31:0]      rwi_bar_data   [0:1];",
			"rwi_bar_cnt        <= 2'd0;",
			"wire bar_push = bar_wr_valid & (rwi_bar_cnt < 2'd2);",
			"wire bar_pop  = (rwi_bar_cnt > 2'd0) & ~rw[RWPOS_CFG_RD_EN] & ~rw[RWPOS_CFG_WR_EN] &",
			"& (rwi_bar_cnt == 2'd0) & ~bar_push & ~in_cmd_read",
			"else if ( bar_pop )",
			"rw[159:128]         <= rwi_bar_data[0];",
			"rw[169:160]         <= rwi_bar_dwaddr[0];",
			"rw[170]             <= 1'b1;",
			"rw[175:172]         <= 4'hf;",
			"if ( bar_push && bar_pop ) begin",
			"end else if ( bar_push ) begin",
			"rwi_bar_cnt                 <= rwi_bar_cnt + 2'd1;",
			"end else if ( bar_pop ) begin",
			"rwi_bar_cnt       <= rwi_bar_cnt - 2'd1;",
		},
	}

	for name, wants := range expected {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		for _, want := range wants {
			if !strings.Contains(string(data), want) {
				t.Errorf("%s is missing %q", name, want)
			}
		}
	}
}

func TestSVPatcherBarMirrorIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	writeBarMirrorFixture(t, dir)

	patcher := NewSVPatcher(firmware.DeviceIDs{}, dir)
	patcher.ShadowConfig = true
	patcher.MSIXCapDword = 0x2C

	if err := patcher.patchShadowConfigSpace(); err != nil {
		t.Fatalf("patchShadowConfigSpace: %v", err)
	}
	if err := patcher.patchBarMirror(); err != nil {
		t.Fatalf("patchBarMirror: %v", err)
	}

	before := make(map[string]string)
	for _, name := range barMirrorFiles {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		before[name] = string(data)
	}

	if err := patcher.patchBarMirror(); err != nil {
		t.Fatalf("second patchBarMirror: %v", err)
	}

	for _, name := range barMirrorFiles {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		if string(data) != before[name] {
			t.Errorf("%s changed on second patchBarMirror run", name)
		}
	}
}

var barMirrorFiles = []string{
	"pcileech_tlps128_cfgspace_shadow.sv",
	"pcileech_pcie_tlp_a7.sv",
	"pcileech_pcie_a7.sv",
	"pcileech_pcie_cfg_a7.sv",
}

func writeBarMirrorFixture(t *testing.T, dir string) {
	t.Helper()
	fixtures := map[string]string{
		"pcileech_tlps128_cfgspace_shadow.sv": `module pcileech_tlps128_cfgspace_shadow #(
    parameter [9:0]         MSIX_CAP_DWORD = 10'h02C
)(
    input                   rst,
    input                   clk_pcie,
    input                   clk_sys,
    IfAXIS128.sink_lite     tlps_in,
    IfAXIS128.source        tlps_cfg_rsp,
    IfShadow2Fifo.shadow    dshadow2fifo
);
    wire                pcie_rx_rden    = tlps_in.tvalid && tlps_in.tuser[0];
    wire                pcie_rx_wren    = tlps_in.tvalid && tlps_in.tuser[0];
    wire [9:0]          pcie_rx_addr    = tlps_in.tdata[75:66];
    wire [31:0]         pcie_rx_data    = tlps_in.tdata[127:96];
    wire [7:0]          pcie_rx_tag     = tlps_in.tdata[47:40];
    wire [3:0]          pcie_rx_be      = tlps_in.tdata[35:32];
    wire [15:0]         pcie_rx_reqid   = tlps_in.tdata[63:48];
endmodule`,
		"pcileech_pcie_tlp_a7.sv": `module pcileech_pcie_tlp_a7(
    input                   rst,
    input                   clk_pcie,
    IfPCIeSignals           ctx
);
    IfAXIS128 tlps_dma();

    pcileech_tlps128_bar_controller i_pcileech_tlps128_bar_controller(
        .cfg_msix_enable        ( ctx.cfg_interrupt_msixenable ),
        .cfg_msix_function_mask ( ctx.cfg_interrupt_msixfm     ),
        .intr_req               ( intr_req                     )
    );

    pcileech_tlps128_cfgspace_shadow i_pcileech_tlps128_cfgspace_shadow(
        .rst            ( rst                           ),
        .tlps_cfg_rsp   ( tlps_cfg_rsp.source           )
    );
endmodule`,
		"pcileech_pcie_a7.sv": `module pcileech_pcie_a7();
    IfPCIeSignals           ctx();
    wire                    intr_req;

    pcileech_pcie_cfg_a7 i_pcileech_pcie_cfg_a7(
        .rst                        ( rst_subsys                ),
        .ctx                        ( ctx                       )
    );

    pcileech_pcie_tlp_a7 i_pcileech_pcie_tlp_a7(
        .rst                        ( rst_subsys                ),
        .ctx                        ( ctx                       )
    );
endmodule`,
		"pcileech_pcie_cfg_a7.sv": `module pcileech_pcie_cfg_a7(
    input                   rst,
    input                   clk_sys,
    input                   clk_pcie
);
    reg     [31:0]      rwi_count_cfgspace_status_cl;

    task pcileech_pcie_cfg_a7_initialvalues;
        begin
            out_wren <= 1'b0;

            rwi_cfg_mgmt_rd_en <= 1'b0;
            rwi_cfg_mgmt_wr_en <= 1'b0;

            // MAGIC
            rw[15:0]    <= 16'h6745;                // +000:
        end
    endtask

    assign in_rden = tickcount64[1] & ~pcie_cfg_rx_almost_full & ( ~rw[RWPOS_CFG_WAIT_COMPLETE] | ~pcie_cfg_rw_en);

    always @ ( posedge clk_pcie )
        if ( rst )
            pcileech_pcie_cfg_a7_initialvalues();
        else
            begin
                // STATUS REGISTER CLEAR
                if ( (rw[RWPOS_CFG_CFGSPACE_STATUS_CL_EN] | rw[RWPOS_CFG_CFGSPACE_COMMAND_EN]) & ~in_cmd_read & ~in_cmd_write & ~rw[RWPOS_CFG_RD_EN] & ~rw[RWPOS_CFG_WR_EN] & ~rwi_cfg_mgmt_rd_en & ~rwi_cfg_mgmt_wr_en )
                    rw[RWPOS_CFG_WR_EN] <= 1'b1;

                // CONFIG SPACE READ/WRITE
                if ( ctx.cfg_mgmt_rd_wr_done )
                    begin
                        rwi_cfg_mgmt_rd_en  <= 1'b0;
                    end
                else if ( rw[RWPOS_CFG_RD_EN] )
                    begin
                        rw[RWPOS_CFG_RD_EN] <= 1'b0;
                    end
                else if ( rw[RWPOS_CFG_WR_EN] )
                    begin
                        rw[RWPOS_CFG_WR_EN] <= 1'b0;
                    end
                    
                // STATIC_TLP TRANSMIT
                if ( 1'b0 ) begin
                end
            end
endmodule`,
	}
	for name, fixture := range fixtures {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(fixture), 0644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
}
