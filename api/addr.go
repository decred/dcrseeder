package api

const (
	// GetAddrsPath is the URL path to fetch a list of public nodes
	GetAddrsPath = "/api/addrs"

	IPVersion       = "ipversion"
	ServiceFlag     = "services"
	ProtocolVersion = "pver"
)

type Node struct {
	Host            string `json:"host"`
	Services        uint64 `json:"services"`
	ProtocolVersion uint32 `json:"pver"`
}
