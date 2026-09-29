package config

type Profile struct {
	URL       string `yaml:"url"`
	User      string `yaml:"user,omitempty"`
	Password  string `yaml:"password,omitempty"`
	Token     string `yaml:"token,omitempty"`
	VerifySSL bool   `yaml:"verify_ssl"`
}

func (p *Profile) MaskedPassword() string {
	if p.Password == "" {
		return ""
	}
	return "***"
}

func (p *Profile) MaskedToken() string {
	if p.Token == "" {
		return ""
	}
	if len(p.Token) <= 8 {
		return "***"
	}
	return p.Token[:8] + "***"
}
