package service

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type PromptConfig struct {
	System                   string `yaml:"system"`
	SystemAppendOnly         string `yaml:"system_append_only"`
	SystemEditOnly           string `yaml:"system_edit_only"`
	VocabularyReference      string `yaml:"vocabulary_reference"`
	AppendInputBuffer        string `yaml:"append_input_buffer"`
	AppendInputBufferNoVocab string `yaml:"append_input_buffer_no_vocab"`
	EditInputBuffer          string `yaml:"edit_input_buffer"`
	TaskTranscribe           string `yaml:"task_transcribe"`
	TaskTranscribeWithVocab  string `yaml:"task_transcribe_with_vocab"`
	TaskAppend               string `yaml:"task_append"`
	TaskAppendNoEmotion      string `yaml:"task_append_no_emotion"`
	TaskEdit                 string `yaml:"task_edit"`
	TaskEditOnly             string `yaml:"task_edit_only"`
	TaskEditNoEmotion        string `yaml:"task_edit_no_emotion"`
	TaskEditOnlyNoEmotion    string `yaml:"task_edit_only_no_emotion"`

	MentionSection        string `yaml:"mention_section"`
	EmotionSection        string `yaml:"emotion_section"`
	EmotionExamples       string `yaml:"emotion_examples"`
	Rule5TitleWithEmotion string `yaml:"rule5_title_with_emotion"`
	Rule5TitleNoEmotion   string `yaml:"rule5_title_no_emotion"`

	Transcribe        string `yaml:"transcribe,omitempty"`
	Modify            string `yaml:"modify,omitempty"`
	AppendContext     string `yaml:"append_context,omitempty"`
	ChatContextSuffix string `yaml:"chat_context_suffix,omitempty"`
}

var activePrompts PromptConfig

func init() {
	ResetPromptsToDefaults()
}

func ResetPromptsToDefaults() {
	activePrompts = PromptConfig{
		System:                   systemPromptTemplate,
		SystemAppendOnly:         systemPromptAppendOnly,
		SystemEditOnly:           systemPromptEditOnly,
		VocabularyReference:      vocabularyReferenceTemplate,
		AppendInputBuffer:        appendInputBufferTemplate,
		AppendInputBufferNoVocab: appendInputBufferNoVocabTemplate,
		EditInputBuffer:          editInputBufferTemplate,
		TaskTranscribe:           taskTranscribe,
		TaskTranscribeWithVocab:  taskTranscribeWithVocab,
		TaskAppend:               taskAppend,
		TaskAppendNoEmotion:      taskAppendNoEmotion,
		TaskEdit:                 taskEdit,
		TaskEditOnly:             taskEditOnly,
		TaskEditNoEmotion:        taskEditNoEmotion,
		TaskEditOnlyNoEmotion:    taskEditOnlyNoEmotion,
		MentionSection:           mentionRecognitionSection,
		EmotionSection:           emotionAnnotationSection,
		EmotionExamples:          emotionExamplesSection,
		Rule5TitleWithEmotion:    rule5TitleWithEmotion,
		Rule5TitleNoEmotion:      rule5TitleNoEmotion,
	}
}

func LoadPrompts(filePath string, logger *zap.Logger) {
	ResetPromptsToDefaults()

	if filePath == "" {
		return
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			if logger != nil {
				logger.Info("voice prompt file not found, using defaults",
					zap.String("path", filePath))
			}
		} else if logger != nil {
			logger.Warn("failed to read voice prompt file, using defaults",
				zap.String("path", filePath), zap.Error(err))
		}
		return
	}

	var cfg PromptConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		if logger != nil {
			logger.Warn("failed to parse voice prompt file, using defaults",
				zap.String("path", filePath), zap.Error(err))
		}
		return
	}

	legacyFields := []struct {
		name  string
		value string
	}{
		{"transcribe", cfg.Transcribe},
		{"modify", cfg.Modify},
		{"append_context", cfg.AppendContext},
		{"chat_context_suffix", cfg.ChatContextSuffix},
	}
	for _, f := range legacyFields {
		if strings.TrimSpace(f.value) != "" && logger != nil {
			logger.Warn("deprecated prompt field ignored",
				zap.String("field", f.name),
				zap.String("hint", "use the new v3 fields instead"))
		}
	}

	if strings.TrimSpace(cfg.System) != "" {
		activePrompts.System = strings.TrimRight(cfg.System, "\r\n")
		if !strings.Contains(activePrompts.System, "{{RULE5_TITLE}}") && logger != nil {
			logger.Warn("custom system prompt lacks {{RULE5_TITLE}} placeholder; emotion toggle will not affect system message",
				zap.String("path", filePath))
		}
	}
	if strings.TrimSpace(cfg.SystemAppendOnly) != "" {
		activePrompts.SystemAppendOnly = strings.TrimRight(cfg.SystemAppendOnly, "\r\n")
	}
	if strings.TrimSpace(cfg.SystemEditOnly) != "" {
		activePrompts.SystemEditOnly = strings.TrimRight(cfg.SystemEditOnly, "\r\n")
	}

	templateFields := []struct {
		name   string
		value  string
		target *string
	}{
		{"vocabulary_reference", cfg.VocabularyReference, &activePrompts.VocabularyReference},
		{"append_input_buffer", cfg.AppendInputBuffer, &activePrompts.AppendInputBuffer},
		{"append_input_buffer_no_vocab", cfg.AppendInputBufferNoVocab, &activePrompts.AppendInputBufferNoVocab},
		{"edit_input_buffer", cfg.EditInputBuffer, &activePrompts.EditInputBuffer},
	}
	for _, f := range templateFields {
		if strings.TrimSpace(f.value) != "" {
			v := strings.TrimRight(f.value, "\r\n")
			if strings.Count(v, "%s") != 1 {
				if logger != nil {
					logger.Warn(f.name+" prompt must contain exactly 1 %s placeholder, using default",
						zap.Int("count", strings.Count(v, "%s")))
				}
			} else {
				*f.target = v
			}
		}
	}

	taskFields := []struct {
		name   string
		value  string
		target *string
	}{
		{"task_transcribe", cfg.TaskTranscribe, &activePrompts.TaskTranscribe},
		{"task_transcribe_with_vocab", cfg.TaskTranscribeWithVocab, &activePrompts.TaskTranscribeWithVocab},
		{"task_append", cfg.TaskAppend, &activePrompts.TaskAppend},
		{"task_append_no_emotion", cfg.TaskAppendNoEmotion, &activePrompts.TaskAppendNoEmotion},
		{"task_edit", cfg.TaskEdit, &activePrompts.TaskEdit},
		{"task_edit_only", cfg.TaskEditOnly, &activePrompts.TaskEditOnly},
		{"task_edit_no_emotion", cfg.TaskEditNoEmotion, &activePrompts.TaskEditNoEmotion},
		{"task_edit_only_no_emotion", cfg.TaskEditOnlyNoEmotion, &activePrompts.TaskEditOnlyNoEmotion},
	}
	for _, f := range taskFields {
		if strings.TrimSpace(f.value) != "" {
			*f.target = strings.TrimRight(f.value, "\r\n")
		}
	}

	emotionPairs := []struct {
		emotionField   string
		emotionValue   string
		noEmotionField string
		noEmotionValue string
	}{
		{"task_append", cfg.TaskAppend, "task_append_no_emotion", cfg.TaskAppendNoEmotion},
		{"task_edit", cfg.TaskEdit, "task_edit_no_emotion", cfg.TaskEditNoEmotion},
		{"task_edit_only", cfg.TaskEditOnly, "task_edit_only_no_emotion", cfg.TaskEditOnlyNoEmotion},
	}
	for _, p := range emotionPairs {
		if strings.TrimSpace(p.emotionValue) != "" && strings.TrimSpace(p.noEmotionValue) == "" && logger != nil {
			logger.Warn("YAML overrides task variant but not its no-emotion counterpart; default will be used when VOICE_EMOTION_EMOJI=false",
				zap.String("override_field", p.emotionField),
				zap.String("missing_field", p.noEmotionField))
		}
	}

	sectionFields := []struct {
		name   string
		value  string
		target *string
	}{
		{"mention_section", cfg.MentionSection, &activePrompts.MentionSection},
		{"emotion_section", cfg.EmotionSection, &activePrompts.EmotionSection},
		{"emotion_examples", cfg.EmotionExamples, &activePrompts.EmotionExamples},
		{"rule5_title_with_emotion", cfg.Rule5TitleWithEmotion, &activePrompts.Rule5TitleWithEmotion},
		{"rule5_title_no_emotion", cfg.Rule5TitleNoEmotion, &activePrompts.Rule5TitleNoEmotion},
	}
	for _, f := range sectionFields {
		if strings.TrimSpace(f.value) != "" {
			*f.target = strings.TrimRight(f.value, "\r\n")
		}
	}

	if logger != nil {
		logger.Info("loaded voice prompts from file",
			zap.String("path", filePath))
	}
}
