package internal

type Workflow struct {
	Name        string  `yaml:"name"`
	Description string  `yaml:"description"`
	Inputs      []Input `yaml:"inputs"`
	Steps       []Step  `yaml:"steps"`
}

type Input struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Required bool   `yaml:"required"`
}

type Step struct {
	ID     string   `yaml:"id"`
	Uses   string   `yaml:"uses"`             // 'browser' | 'shell' | 'api'
	Prompt string   `yaml:"prompt,omitempty"` // if (uses: browser) prompt template
	Run    string   `yaml:"run"`              // (if uses: shell) command line
	Call   *ApiCall `yaml:"call"`             // (if uses: api)
}
type ApiCall struct {
	Method  string            `yaml:"method"`
	Url     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers"`
	Body    map[string]any    `yaml:"body"`
}

type SteelSession struct {
	ID               string `json:"id"`
	SessionViewerURL string `json:"session_viewer_url"`
}
