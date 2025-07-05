package config

// Schema represents the JSON schema
type Schema = Command

type Command struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Commands    []*Command  `json:"commands"`
	Examples    []*Example  `json:"examples"`
	Options     []*Option   `json:"options"`
	Arguments   []*Argument `json:"arguments"`
	Output      *Output     `json:"output"`
}

type Argument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Default     string `json:"default"`
	Multiline   bool   `json:"multiline"`
}

type Example struct {
	Usage       string `json:"usage"`
	Description string `json:"description"`
}

type Option struct {
	Flags       string `json:"flags"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Default     string `json:"default"`
	Multiline   bool   `json:"multiline"`
}

type Output struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}
