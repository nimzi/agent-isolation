package aishell

const (
	DefaultContainerBase = "ai-agent-shell"
	DefaultImage         = "ai-agent-shell"
	DefaultVolumeBase    = "ai_agent_shell_home"

	LabelNS       = "com.nimzi.ai-shell"
	LabelManaged  = LabelNS + ".managed"
	LabelSchema   = LabelNS + ".schema"
	LabelInstance = LabelNS + ".instance"
	LabelVolume   = LabelNS + ".volume"
	// Note: workdir is NOT stored as a label; it's discovered from the /work bind mount.
)
