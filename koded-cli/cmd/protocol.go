package cmd

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// ── protocol subcommand ───────────────────────────────────────────────────────

var protocolCmd = &cobra.Command{
	Use:   "protocol",
	Short: "HTTP/K.0 protocol tools",
	Long: `Tools for testing and inspecting HTTP/K.0 connections.

  koded protocol ping <addr>     → test reachability of a K.0 server
  koded protocol info            → show local protocol info
  koded protocol bench <addr>    → run a quick latency benchmark`,
}

func init() {
	rootCmd.AddCommand(protocolCmd)
	protocolCmd.AddCommand(protocolPingCmd)
	protocolCmd.AddCommand(protocolInfoCmd)
	protocolCmd.AddCommand(protocolBenchCmd)
}

// ── protocol ping ─────────────────────────────────────────────────────────────

var pingCount int
var pingTimeout int

var protocolPingCmd = &cobra.Command{
	Use:   "ping <addr>",
	Short: "Send HTTP/K.0 handshake probes to a server",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		addr := args[0]

		fmt.Printf("\n  %s %s\n\n",
			color(colorCyan, "HTTP/K.0 PING"),
			color(colorGray, addr),
		)

		conn, err := net.Dial("udp", addr)
		if err != nil {
			fmt.Printf("  %s Cannot reach %s: %v\n", color(colorRed, "✗"), addr, err)
			os.Exit(1)
		}
		defer conn.Close()

		var totalRTT time.Duration
		var success int

		for i := 0; i < pingCount; i++ {
			connID := randomU64()
			pkt    := buildInitPacket(connID)

			start := time.Now()
			conn.SetWriteDeadline(time.Now().Add(time.Duration(pingTimeout) * time.Second))
			conn.SetReadDeadline(time.Now().Add(time.Duration(pingTimeout) * time.Second))

			_, err := conn.Write(pkt)
			if err != nil {
				fmt.Printf("  %s probe %d: send error: %v\n", color(colorRed, "✗"), i+1, err)
				continue
			}

			buf := make([]byte, 256)
			_, err  = conn.Read(buf)
			rtt    := time.Since(start)

			if err != nil {
				fmt.Printf("  %s probe %d: timeout\n", color(colorYellow, "!"), i+1)
			} else {
				// Check for INIT_ACK (frame_type byte = 0x01)
				frameType := uint8(0)
				if len(buf) > 17 { frameType = buf[17] }
				status := color(colorGray, "?")
				if frameType == 0x01 {
					status = color(colorGreen, "INIT_ACK")
				}
				fmt.Printf("  %s probe %d: rtt=%s  %s\n",
					color(colorGreen, "✓"), i+1, rtt.Round(time.Microsecond), status,
				)
				totalRTT += rtt
				success++
			}

			if i < pingCount-1 {
				time.Sleep(500 * time.Millisecond)
			}
		}

		fmt.Printf("\n  %s %d/%d probes received",
			color(colorGray, "---"),
			success, pingCount,
		)
		if success > 0 {
			avg := totalRTT / time.Duration(success)
			fmt.Printf("  avg=%s", avg.Round(time.Microsecond))
		}
		fmt.Printf("\n\n")
	},
}

func init() {
	protocolPingCmd.Flags().IntVarP(&pingCount,   "count",   "c", 4,  "Number of probes to send")
	protocolPingCmd.Flags().IntVarP(&pingTimeout, "timeout", "t", 3,  "Timeout per probe (seconds)")
}

// ── protocol info ─────────────────────────────────────────────────────────────

var protocolInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show HTTP/K.0 protocol information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(color(colorCyan, `
  HTTP/K.0 Protocol
  ─────────────────
`))
		info := [][]string{
			{"Transport",    "UDP (QUIC-inspired)"},
			{"Security",     "TLS 1.3 / ChaCha20-Poly1305 per-packet AEAD"},
			{"Multiplexing", "Independent streams (no head-of-line blocking)"},
			{"Reliability",  "Per-stream flag (reliable = ACK+retransmit, partial = FEC)"},
			{"Congestion",   "BBR-lite (bandwidth/delay based, not loss based)"},
			{"FEC",          "XOR parity, 4 data + 1 parity per block"},
			{"0-RTT",        "Session ticket resumption (safe methods only)"},
			{"Migration",    "PATH_CHALLENGE/RESPONSE for IP changes"},
			{"Frame types",  "HEADERS, DATA, TRAILERS, RESET, PING, GOAWAY"},
			{"Compression",  "HPACK-lite (full 61-entry RFC 7541 static table)"},
			{"Flow control", "Credit-based per-stream + connection windows"},
			{"Port",         "9000 (default)"},
		}
		for _, row := range info {
			fmt.Printf("  %-16s %s\n",
				color(colorGray, row[0]+":"),
				row[1],
			)
		}
		fmt.Println()

		fmt.Print(color(colorGray, `  Wire format (per packet):
  [8B conn_id][8B packet_num][1B flags][1B frame_type][4B payload_len][payload]

  Flags byte:
    bit 0 = ordered     bit 1 = reliable     bit 2 = fec_parity

`))
	},
}

// ── protocol bench ────────────────────────────────────────────────────────────

var benchN int

var protocolBenchCmd = &cobra.Command{
	Use:   "bench <addr>",
	Short: "Run a quick RTT latency benchmark against a K.0 server",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		addr := args[0]

		fmt.Printf("\n  %s %s  (%d probes)\n\n",
			color(colorCyan, "HTTP/K.0 BENCH"),
			color(colorGray, addr),
			benchN,
		)

		conn, err := net.Dial("udp", addr)
		if err != nil {
			fmt.Printf("  %s %v\n", color(colorRed, "✗"), err)
			os.Exit(1)
		}
		defer conn.Close()

		var rtts []time.Duration

		for i := 0; i < benchN; i++ {
			connID := randomU64()
			pkt    := buildInitPacket(connID)
			conn.SetDeadline(time.Now().Add(3 * time.Second))

			start := time.Now()
			conn.Write(pkt)
			buf := make([]byte, 256)
			conn.Read(buf)
			rtts = append(rtts, time.Since(start))

			// Print live progress bar
			pct := float64(i+1) / float64(benchN)
			w   := 30
			bar := fmt.Sprintf("[%-*s]", w, repeatStr("=", int(pct*float64(w))))
			fmt.Printf("\r  %s %.0f%%", bar, pct*100)
		}
		fmt.Println()

		// Stats
		var total time.Duration
		min, max := rtts[0], rtts[0]
		for _, r := range rtts {
			total += r
			if r < min { min = r }
			if r > max { max = r }
		}
		avg := total / time.Duration(len(rtts))

		fmt.Printf("\n  %s\n", color(colorGray, "Results:"))
		fmt.Printf("  %-8s %s\n", color(colorGray, "min:"), min.Round(time.Microsecond))
		fmt.Printf("  %-8s %s\n", color(colorGray, "avg:"), avg.Round(time.Microsecond))
		fmt.Printf("  %-8s %s\n", color(colorGray, "max:"), max.Round(time.Microsecond))
		fmt.Printf("  %-8s %d probes\n\n", color(colorGray, "count:"), len(rtts))
	},
}

func init() {
	protocolBenchCmd.Flags().IntVarP(&benchN, "count", "c", 20, "Number of probes")
}

// ── K.0 packet helpers ────────────────────────────────────────────────────────
// Builds a minimal HTTP/K.0 INIT packet for ping/bench.
// Matches the wire format from the protocol implementation exactly.

func buildInitPacket(connID uint64) []byte {
	// [8B conn_id][8B packet_num][1B flags][1B frame_type=0x00][4B payload_len][payload]
	payload := []byte("K0_INIT")
	pkt     := make([]byte, 22+len(payload))
	binary.BigEndian.PutUint64(pkt[0:8],  connID)
	binary.BigEndian.PutUint64(pkt[8:16], 0)     // packet_num = 0
	pkt[16] = 0                                    // flags
	pkt[17] = 0x00                                 // HandshakeInit
	binary.BigEndian.PutUint32(pkt[18:22], uint32(len(payload)))
	copy(pkt[22:], payload)
	return pkt
}

func randomU64() uint64 {
	var b [8]byte
	rand.Read(b[:])
	return binary.BigEndian.Uint64(b[:])
}

func repeatStr(s string, n int) string {
	if n <= 0 { return "" }
	result := ""
	for i := 0; i < n; i++ { result += s }
	return result
}