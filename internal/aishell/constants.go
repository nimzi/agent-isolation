package aishell

const (
	DefaultContainerBase = "ai-agent-shell"
	DefaultImage         = "ai-agent-shell"
	DefaultVolumeBase    = "ai_agent_shell_home"

	LabelNS       = "com.nimzi.ai-shell"
	LabelManaged  = LabelNS + ".managed"
	LabelSchema   = LabelNS + ".schema"
	LabelWorkdir  = LabelNS + ".workdir"
	LabelInstance = LabelNS + ".instance"
	LabelVolume   = LabelNS + ".volume"
)
