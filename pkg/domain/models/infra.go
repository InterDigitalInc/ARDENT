package models

// A Repository interface provides function signatures for storage related operations.
type Repository interface {

	// interface in this call can be an array of different structs like Compute,
	// Switch etc., but at a time all the elements will be of single type.
	Add(e []interface{}) error

	// Argument to this interface will be any Entity like Compute, Network etc.
	// Any parameter set inside the Entity will be used to delete only those
	// entities from storage that will satisfy the parameter value.
	// for removing matching entries.
	Remove(es interface{}) error

	// Query struct will be used to form condition to retrieve values from storage.
	Get(q *Query) (interface{}, error)
}

// A Query is used to form an SQL query to interact with storage.
type Query struct {
	// Entity to be fetched, E.g.: Network{}
	Entity interface{}

	// expr (if any - in string format) to apply while fetching the entity.
	// For ex: Compute.Id='CN1'
	// Expression can contain condition only from single table.
	Expr []interface{}
}

// Following structures and their definitions may change based on schema.

// A Compute will contain all Compute node related properties.
type Compute struct {
	Name      string   `yaml:"name"`
	AvailZone string   `yaml:"availability_zone"`
	Vcpus     int      `yaml:"vcpus"`
	RAM       int      `yaml:"ram"`
	Disk      int      `yaml:"disk"`
	Networks  []string `yaml:"networks" entity.ukey:"Network.Identifier"`
	Tier      string   `yaml:"tier"`
}

// A Network will contain all OpenStack network related properties.
type Network struct {
	Identifier string `yaml:"identifier"`
	Category   string `yaml:"category"`
}

// A Subnet will contain all OpenStack subnet related properties.
type Subnet struct {
	Identifier string `yaml:"identifier"`
	Category   string `yaml:"category"`
}

// An InfraService will contain properties related to Infrastructure Service.
// For InfraServices, this copy is required, otherwise for other structures
// yaml parser filled structures are directly used for passing to Storage layer.
type InfraService struct {
	ServiceType string // Service types are fixed - 'dns', 'sdn_controller'
	Value       string
}

// An InfraServices provides YAML filled structures that will be
// converted to InfraService for adding it to DB.
type InfraServices struct {
	DNS           string `yaml:"dns"            serviceType:"dns"`
	SdnController string `yaml:"sdn_controller" serviceType:"sdn_controller"`
}

// A SecurityGroup will contain properties related to Security Group.
type SecurityGroup struct {
	Identifier string `yaml:"identifier"`
	Category   string `yaml:"category"`
}

// A Config will contain configuration related values.
// For Metadata, this copy is required, otherwise for other structures
// yaml parser filled structures are directly used for passing to Storage layer.
type Config struct {
	ConfKey string // Configuration keys are fixed - 'ardent-version', 'cidr', 'mtu', 'os-cli-version', 'os-tenant-id'
	Value   string
}

// A Metadata provides YAML filled structures that will be converted
// to Config for adding it to DB.
type Metadata struct {
	Tenant        string `yaml:"tenant"          confKey:"os-tenant-id"`
	Cidr          string `yaml:"cidr"            confKey:"cidr"`
	Mtu           int    `yaml:"mtu"             confKey:"mtu"`
	SiaIpFrontend string `yaml:"sia-ip-frontend" confKey:"sia-ip-frontend"`
	Ipv4Rules     string `yaml:"ipv4-rules"      confKey:"enable-ipv4-rules"`
	DhcpAgents    int    `yaml:"dhcp_agents"     confKey:"dhcp_agents"`
}

// Flavor will contain all Flavor node related properties.
type Flavor struct {
	Name  string
	Vcpus int
	RAM   int
	Disk  int
}

// SecurityGpRule will contain all SecurityGpRule node related properties.
type SecurityGrpRule struct {
	Name     string
	Protocol string
	Port     int
}

// And implements joining of two SQL queries using 'AND' keyword.
//
// Parameters:
//  op: Operand of interface type constituted of an SQL query.
//
// Returns:
//  Nil.
func (q *Query) And(op interface{}) {
	q.Expr = append(q.Expr, "AND")
	q.Expr = append(q.Expr, op)
}

// Or implements joining of two SQL queries using 'OR' keyword.
//
// Parameters:
//  op: Operand of interface type constituted of an SQL query.
//
// Returns:
//  Nil.
func (q *Query) Or(op interface{}) {
	q.Expr = append(q.Expr, "OR")
	q.Expr = append(q.Expr, op)
}
