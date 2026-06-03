package service

import (
	"strings"
	"testing"
)

func TestBuildSystemMessage_WithEmotion(t *testing.T) {
	ResetPromptsToDefaults()
	msg := BuildSystemMessage(true, false)

	if !strings.Contains(msg, "情绪标注") {
		t.Error("expected emotion section in system message")
	}
	if !strings.Contains(msg, "@提及识别") {
		t.Error("expected mention section in system message")
	}
	if !strings.Contains(msg, "语气保真与情绪标注") {
		t.Error("expected rule5 title with emotion")
	}
}

func TestBuildSystemMessage_NoEmotion(t *testing.T) {
	ResetPromptsToDefaults()
	msg := BuildSystemMessage(false, false)

	if strings.Contains(msg, "情绪标注（⚠️ 必须执行）") {
		t.Error("expected no emotion section")
	}
	if !strings.Contains(msg, "@提及识别") {
		t.Error("expected mention section")
	}
	if strings.Contains(msg, "语气保真与情绪标注") {
		t.Error("expected rule5 title without emotion keyword")
	}
	if !strings.Contains(msg, "语气保真") {
		t.Error("expected rule5 title")
	}
}

func TestBuildSystemMessage_SkipMention(t *testing.T) {
	ResetPromptsToDefaults()
	msg := BuildSystemMessage(true, true)

	if strings.Contains(msg, "@提及识别") {
		t.Error("expected no mention section when skipMention=true")
	}
}

func TestBuildSystemMessage_BroadcastMention(t *testing.T) {
	ResetPromptsToDefaults()
	msg := BuildSystemMessage(true, false)

	if !strings.Contains(msg, "### 广播 @mention") {
		t.Error("expected broadcast mention section in system message")
	}
	if !strings.Contains(msg, "@所有人") {
		t.Error("expected @所有人 broadcast token guidance")
	}
	if !strings.Contains(msg, "@所有AI") {
		t.Error("expected @所有AI broadcast token guidance")
	}
	if strings.Contains(msg, "@所有 AI") {
		t.Error("broadcast token must be atomic: @所有AI must never contain a space (@所有 AI)")
	}
}

func TestBuildSystemMessage_SkipBroadcastMention(t *testing.T) {
	ResetPromptsToDefaults()
	msg := BuildSystemMessage(true, true)

	if strings.Contains(msg, "### 广播 @mention") {
		t.Error("expected no broadcast mention section when skipMention=true")
	}
}

func TestBuildSystemMessage_EditOnlyMention(t *testing.T) {
	ResetPromptsToDefaults()
	msg := BuildSystemMessage(true, false, "edit_only")

	if !strings.Contains(msg, "@提及识别") {
		t.Error("expected mention section in edit_only system message")
	}
	if !strings.Contains(msg, "### 广播 @mention") {
		t.Error("expected broadcast mention section in edit_only system message")
	}
}

func TestBuildSystemMessage_EditOnlySkipMention(t *testing.T) {
	ResetPromptsToDefaults()
	msg := BuildSystemMessage(true, true, "edit_only")

	if strings.Contains(msg, "@提及识别") {
		t.Error("expected no mention section in edit_only when skipMention=true")
	}
}

func TestBuildSystemMessage_EditOnlyWithEmotion(t *testing.T) {
	ResetPromptsToDefaults()
	msg := BuildSystemMessage(true, false, "edit_only")

	if !strings.Contains(msg, "### 规则 5：情绪标注") {
		t.Error("expected emotion annotation rule (rule 5) in edit_only system message when emotion enabled")
	}
	if !strings.Contains(msg, "情绪标注（⚠️ 必须执行）") {
		t.Error("expected emotion annotation section body in edit_only system message when emotion enabled")
	}
}

func TestBuildSystemMessage_EditOnlyNoEmotion(t *testing.T) {
	ResetPromptsToDefaults()
	msg := BuildSystemMessage(false, false, "edit_only")

	// When emotion is disabled, the entire rule 5 (heading + intro + section)
	// must be gone, not just the placeholder body. Guards against regression
	// where the rule 5 heading/intro was hardcoded in systemPromptEditOnly.
	if strings.Contains(msg, "### 规则 5：情绪标注") {
		t.Error("expected no emotion annotation rule (rule 5) in edit_only when emotion disabled")
	}
	if strings.Contains(msg, "情绪标注（⚠️ 必须执行）") {
		t.Error("expected no emotion annotation section in edit_only when emotion disabled")
	}
}

func TestBuildSystemMessage_FallbackToCustomSystem(t *testing.T) {
	ResetPromptsToDefaults()
	activePrompts.System = "custom system prompt"
	activePrompts.SystemOverridden = true
	// AppendOnlyOverridden and EditOnlyOverridden remain false

	appendMsg := BuildSystemMessage(true, false, "append")
	if appendMsg != "custom system prompt" {
		t.Errorf("append mode should fall back to custom system prompt, got: %s", appendMsg[:50])
	}

	editOnlyMsg := BuildSystemMessage(true, false, "edit_only")
	if editOnlyMsg != "custom system prompt" {
		t.Errorf("edit_only mode should fall back to custom system prompt, got: %s", editOnlyMsg[:50])
	}
}

func TestBuildUserMessage_AppendNoContext(t *testing.T) {
	ResetPromptsToDefaults()
	msg := BuildUserMessage("append", "", "", true)

	if !strings.Contains(msg, "请转写音频中的语音") {
		t.Error("expected transcribe task")
	}
}

func TestBuildUserMessage_AppendWithContext(t *testing.T) {
	ResetPromptsToDefaults()
	msg := BuildUserMessage("append", "existing text", "", true)

	if !strings.Contains(msg, "<input_buffer>") {
		t.Error("expected input_buffer tag")
	}
	if !strings.Contains(msg, "existing text") {
		t.Error("expected context text")
	}
}

func TestBuildUserMessage_AppendWithVocab(t *testing.T) {
	ResetPromptsToDefaults()
	msg := BuildUserMessage("append", "existing", "vocab data", true)

	if !strings.Contains(msg, "<vocabulary_reference>") {
		t.Error("expected vocabulary_reference tag")
	}
	if !strings.Contains(msg, "vocab data") {
		t.Error("expected vocabulary content")
	}
}

func TestBuildUserMessage_EditWithContext(t *testing.T) {
	ResetPromptsToDefaults()
	msg := BuildUserMessage("edit", "text to edit", "", true)

	if !strings.Contains(msg, "当前已有的文本") {
		t.Error("expected edit input buffer template")
	}
	if !strings.Contains(msg, "text to edit") {
		t.Error("expected context text")
	}
	if !strings.Contains(msg, "编辑指令") {
		t.Error("expected edit task instruction")
	}
}

func TestBuildUserMessage_EditOnly(t *testing.T) {
	ResetPromptsToDefaults()
	msg := BuildUserMessage("edit_only", "text to edit", "", true)

	if !strings.Contains(msg, "根据音频语音指令编辑上方 input_buffer") {
		t.Error("expected edit_only task instruction")
	}
}

func TestBuildUserMessage_EditNoEmotion(t *testing.T) {
	ResetPromptsToDefaults()
	msg := BuildUserMessage("edit", "text", "", false)

	if strings.Contains(msg, "情绪标注") {
		t.Error("expected no emotion annotation in no-emotion mode")
	}
	if !strings.Contains(msg, "语气保真") {
		t.Error("expected tone preservation")
	}
}

func TestBuildUserMessage_DefaultMode(t *testing.T) {
	ResetPromptsToDefaults()
	msg := BuildUserMessage("", "", "", true)

	if !strings.Contains(msg, "请转写音频中的语音") {
		t.Error("expected transcribe task for default mode")
	}
}

func TestBuildVocabularyReference_NoPersonalNoMember(t *testing.T) {
	result := BuildVocabularyReference("", "", "chat data")
	if result != "chat data" {
		t.Errorf("expected plain chat context, got %q", result)
	}
}

func TestBuildVocabularyReference_WithPersonal(t *testing.T) {
	result := BuildVocabularyReference("personal ctx", "", "chat data")
	if !strings.Contains(result, "<personal_vocabulary>") {
		t.Error("expected personal_vocabulary tag")
	}
	if !strings.Contains(result, "<latest_chat_context>") {
		t.Error("expected latest_chat_context tag")
	}
}

func TestBuildVocabularyReference_WithMember(t *testing.T) {
	result := BuildVocabularyReference("", "member ctx", "")
	if !strings.Contains(result, "<member_vocabulary>") {
		t.Error("expected member_vocabulary tag")
	}
}

func TestBuildVocabularyReference_All(t *testing.T) {
	result := BuildVocabularyReference("personal", "member", "chat")
	if !strings.Contains(result, "<personal_vocabulary>") {
		t.Error("expected personal_vocabulary")
	}
	if !strings.Contains(result, "<member_vocabulary>") {
		t.Error("expected member_vocabulary")
	}
	if !strings.Contains(result, "<latest_chat_context>") {
		t.Error("expected latest_chat_context")
	}
}

func TestIsNoSpeech(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"", true},
		{"[NO_SPEECH]", true},
		{"  [NO_SPEECH]  ", true},
		{"some text [NO_SPEECH] more", true},
		{"hello world", false},
		{"NOSPEECH", false},
	}

	for _, tt := range tests {
		got := IsNoSpeech(tt.text)
		if got != tt.want {
			t.Errorf("IsNoSpeech(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}
