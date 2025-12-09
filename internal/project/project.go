package project

// Project represents a validated deployment project configuration
type Project struct {
	Name                string
	Path                string
	Secret              string
	Branch              string
	PullTimeout         int
	PostDeployTimeout   int
	PostDeploy          []interface{} // Can be string or []string
	PostActivateTimeout int
	PostActivate        []interface{} // Can be string or []string
}

// ProjectConfig represents the YAML configuration for a project
type ProjectConfig struct {
	Path                string        `yaml:"path"`
	Secret              string        `yaml:"secret"`
	Branch              string        `yaml:"branch"`
	PullTimeout         int           `yaml:"pull_timeout"`
	PostDeployTimeout   int           `yaml:"post_deploy_timeout"`
	PostDeploy          []interface{} `yaml:"post_deploy"`
	PostActivateTimeout int           `yaml:"post_activate_timeout"`
	PostActivate        []interface{} `yaml:"post_activate"`
}

// Config represents the root configuration structure
type Config struct {
	Projects map[string]ProjectConfig `yaml:"projects"`
}
