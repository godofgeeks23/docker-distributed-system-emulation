package topology

type Region struct {
	RouterService string
	ProbeService  string
	Subnet        string
	ProbeIP       string
}

var Regions = map[string]Region{
	"us-east": {
		RouterService: "router-us-east",
		ProbeService:  "probe-us-east",
		Subnet:        "10.10.1.0/24",
		ProbeIP:       "10.10.1.10",
	},
	"eu-west": {
		RouterService: "router-eu-west",
		ProbeService:  "probe-eu-west",
		Subnet:        "10.10.2.0/24",
		ProbeIP:       "10.10.2.10",
	},
	"ap-south": {
		RouterService: "router-ap-south",
		ProbeService:  "probe-ap-south",
		Subnet:        "10.10.3.0/24",
		ProbeIP:       "10.10.3.10",
	},
}

var RegionOrder = []string{"us-east", "eu-west", "ap-south"}
