// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Station's fixed OpenAI plugin with proper tool calling support
package genkit

import (
	"context"
	"fmt"
	"os"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core"
	"github.com/firebase/genkit/go/genkit"
	openaiGo "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

const provider = "openai"

type TextEmbeddingConfig struct {
	Dimensions     int                                       `json:"dimensions,omitempty"`
	EncodingFormat openaiGo.EmbeddingNewParamsEncodingFormat `json:"encodingFormat,omitempty"`
}

// EmbedderRef represents the main structure for an embedding model's definition.
type EmbedderRef struct {
	Name         string
	ConfigSchema TextEmbeddingConfig // Represents the schema, can be used for default config
}

var (
	// Supported models: https://platform.openai.com/docs/models
	supportedModels = map[string]ai.ModelInfo{
		"gpt-5": {
			Label:    "OpenAI GPT-5 (Station Fixed)",
			Supports: &Multimodal,
			Versions: []string{"gpt-5"},
		},
		"gpt-4o": {
			Label:    "OpenAI GPT-4o (Station Fixed)",
			Supports: &Multimodal,
			Versions: []string{"gpt-4o", "gpt-4o-2024-11-20", "gpt-4o-2024-08-06", "gpt-4o-2024-05-13"},
		},
		"gpt-4o-mini": {
			Label:    "OpenAI GPT-4o-mini (Station Fixed)",
			Supports: &Multimodal,
			Versions: []string{"gpt-4o-mini", "gpt-4o-mini-2024-07-18"},
		},
		"gpt-4-turbo": {
			Label:    "OpenAI GPT-4-turbo (Station Fixed)",
			Supports: &Multimodal,
			Versions: []string{"gpt-4-turbo", "gpt-4-turbo-2024-04-09", "gpt-4-turbo-preview", "gpt-4-0125-preview"},
		},
		"gpt-4": {
			Label: "OpenAI GPT-4 (Station Fixed)",
			Supports: &ai.ModelSupports{
				Multiturn:  true,
				Tools:      true, // Station fixes enable tool calling
				SystemRole: true,
				Media:      false,
			},
			Versions: []string{"gpt-4", "gpt-4-0613", "gpt-4-0314"},
		},
		"gpt-3.5-turbo": {
			Label: "OpenAI GPT-3.5-turbo (Station Fixed)",
			Supports: &ai.ModelSupports{
				Multiturn:  true,
				Tools:      true, // Station fixes enable tool calling
				SystemRole: true,
				Media:      false,
			},
			Versions: []string{"gpt-3.5-turbo", "gpt-3.5-turbo-0125", "gpt-3.5-turbo-1106", "gpt-3.5-turbo-instruct"},
		},
		openaiGo.ChatModelO3Mini: {
			Label:    "OpenAI o3-mini (Station Fixed)",
			Supports: &BasicText,
			Versions: []string{"o3-mini", "o3-mini-2025-01-31"},
		},
		openaiGo.ChatModelO1: {
			Label:    "OpenAI o1 (Station Fixed)",
			Supports: &BasicText,
			Versions: []string{"o1", "o1-2024-12-17"},
		},
		openaiGo.ChatModelO1Preview: {
			Label: "OpenAI o1-preview (Station Fixed)",
			Supports: &ai.ModelSupports{
				Multiturn:  true,
				Tools:      false,
				SystemRole: false,
				Media:      false,
			},
			Versions: []string{"o1-preview", "o1-preview-2024-09-12"},
		},
		openaiGo.ChatModelO1Mini: {
			Label: "OpenAI o1-mini (Station Fixed)",
			Supports: &ai.ModelSupports{
				Multiturn:  true,
				Tools:      false,
				SystemRole: false,
				Media:      false,
			},
			Versions: []string{"o1-mini", "o1-mini-2024-09-12"},
		},
	}

	// Embedding models disabled for now due to API compatibility issues
	// Can be re-enabled later when needed
)

// StationOpenAI provides Station's fixed OpenAI plugin with proper tool calling
type StationOpenAI struct {
	// APIKey is the API key for the OpenAI API. If empty, the values of the environment variable "OPENAI_API_KEY" will be consulted.
	// Request a key at https://platform.openai.com/api-keys
	APIKey string
	// Optional: BaseURL allows using OpenAI-compatible APIs
	BaseURL string
	// Optional: Opts are additional options for the OpenAI client.
	// Can include other options like WithOrganization, WithBaseURL, etc.
	Opts []option.RequestOption

	stationOpenAICompatible *StationOpenAICompatible
}

// Name implements genkit.Plugin.
func (o *StationOpenAI) Name() string {
	return provider
}

// Init implements genkit.Plugin.
func (o *StationOpenAI) Init(ctx context.Context, g *genkit.Genkit) error {
	apiKey := o.APIKey

	// if api key is not set, get it from environment variable
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	if apiKey == "" {
		return fmt.Errorf("station openai plugin initialization failed: apiKey is required (set OPENAI_API_KEY env var)")
	}

	if o.stationOpenAICompatible == nil {
		o.stationOpenAICompatible = &StationOpenAICompatible{}
	}

	// set the options
	o.stationOpenAICompatible.Opts = []option.RequestOption{
		option.WithAPIKey(apiKey),
	}

	// Add base URL if provided (for OpenAI-compatible APIs)
	if o.BaseURL != "" {
		o.stationOpenAICompatible.Opts = append(o.stationOpenAICompatible.Opts, option.WithBaseURL(o.BaseURL))
	}

	if len(o.Opts) > 0 {
		o.stationOpenAICompatible.Opts = append(o.stationOpenAICompatible.Opts, o.Opts...)
	}

	o.stationOpenAICompatible.Provider = provider
	if err := o.stationOpenAICompatible.Init(ctx, g); err != nil {
		return err
	}

	// define default models with Station's fixes
	for model, info := range supportedModels {
		if _, err := o.DefineModel(g, model, info); err != nil {
			return err
		}
	}

	// Embedders disabled for now due to API compatibility issues

	return nil
}

func (o *StationOpenAI) Model(g *genkit.Genkit, name string) ai.Model {
	return o.stationOpenAICompatible.Model(g, name, provider)
}

func (o *StationOpenAI) DefineModel(g *genkit.Genkit, name string, info ai.ModelInfo) (ai.Model, error) {
	return o.stationOpenAICompatible.DefineModel(g, provider, name, info)
}

// Embedder functionality temporarily disabled due to API compatibility issues

func (o *StationOpenAI) ListActions(ctx context.Context) []core.ActionDesc {
	return o.stationOpenAICompatible.ListActions(ctx)
}

func (o *StationOpenAI) ResolveAction(g *genkit.Genkit, atype core.ActionType, name string) error {
	return o.stationOpenAICompatible.ResolveAction(g, atype, name)
}