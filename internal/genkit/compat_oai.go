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

// Package genkit provides Station's custom genkit OpenAI compatibility plugin
// with fixes for the tool_call_id bug in multi-turn agent conversations.
//
// This is a copy of Firebase Genkit's compat_oai plugin with critical fixes
// for proper tool calling protocol compliance with OpenAI's API.
//
// Key Fix: Uses ToolRequest.Ref instead of ToolRequest.Name as tool_call_id
// to prevent tool execution results from being used as identifiers.
package genkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core"
	"github.com/firebase/genkit/go/genkit"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

var (
	// BasicText describes model capabilities for text-only GPT models.
	BasicText = ai.ModelSupports{
		Multiturn:  true,
		Tools:      true,
		SystemRole: true,
		Media:      false,
	}

	// Multimodal describes model capabilities for multimodal GPT models.
	Multimodal = ai.ModelSupports{
		Multiturn:  true,
		Tools:      true,
		SystemRole: true,
		Media:      true,
		ToolChoice: true,
	}
)

// StationOpenAICompatible is Station's custom OpenAI compatibility plugin.
// It provides the same functionality as Firebase Genkit's compat_oai plugin
// but with critical fixes for tool calling in multi-turn conversations.
type StationOpenAICompatible struct {
	// mu protects concurrent access to the client and initialization state
	mu sync.Mutex

	// initted tracks whether the plugin has been initialized
	initted bool

	// client is the OpenAI client used for making API requests
	// see https://github.com/openai/openai-go
	client *openai.Client

	// Opts contains request options for the OpenAI client.
	// Required: Must include at least WithAPIKey for authentication.
	// Optional: Can include other options like WithOrganization, WithBaseURL, etc.
	Opts []option.RequestOption

	// Provider is a unique identifier for the plugin.
	// This will be used as a prefix for model names (e.g., "station-openai/model-name").
	// Should be lowercase and match the plugin's Name() method.
	Provider string
	
	// LogCallback allows progressive logging during model execution
	LogCallback func(map[string]interface{})
}

// Init implements genkit.Plugin.
func (o *StationOpenAICompatible) Init(ctx context.Context, g *genkit.Genkit) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.initted {
		return errors.New("station openai compat plugin already initialized")
	}

	// create client
	client := openai.NewClient(o.Opts...)
	o.client = &client
	o.initted = true

	return nil
}

// Name implements genkit.Plugin.
func (o *StationOpenAICompatible) Name() string {
	return o.Provider
}

// SetLogCallback sets the logging callback for progressive logging during model execution
func (o *StationOpenAICompatible) SetLogCallback(callback func(map[string]interface{})) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.LogCallback = callback
}

// DefineModel defines a model in the registry
func (o *StationOpenAICompatible) DefineModel(g *genkit.Genkit, provider, name string, info ai.ModelInfo) (ai.Model, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.initted {
		return nil, errors.New("StationOpenAICompatible.Init not called")
	}

	// Strip provider prefix if present to check against supportedModels
	modelName := strings.TrimPrefix(name, provider+"/")

	return genkit.DefineModel(g, provider, name, &info, func(
		ctx context.Context,
		input *ai.ModelRequest,
		cb func(context.Context, *ai.ModelResponseChunk) error,
	) (*ai.ModelResponse, error) {
		// Configure the response generator with input using Station's fixed version
		generator := NewStationModelGenerator(o.client, modelName).WithMessages(input.Messages).WithConfig(input.Config).WithTools(input.Tools)
		
		// Add logging callback if available
		if o.LogCallback != nil {
			generator = generator.WithLogCallback(o.LogCallback)
		}

		// Generate response
		resp, err := generator.Generate(ctx, cb)
		if err != nil {
			return nil, err
		}

		return resp, nil
	}), nil
}

// Embedder functionality temporarily disabled due to API compatibility issues

// Model returns the [ai.Model] with the given name.
// It returns nil if the model was not defined.
func (o *StationOpenAICompatible) Model(g *genkit.Genkit, name string, provider string) ai.Model {
	return genkit.LookupModel(g, provider, name)
}

// IsDefinedModel reports whether the named [Model] is defined by this plugin.
func (o *StationOpenAICompatible) IsDefinedModel(g *genkit.Genkit, name string, provider string) bool {
	return genkit.LookupModel(g, provider, name) != nil
}

func (o *StationOpenAICompatible) ListActions(ctx context.Context) []core.ActionDesc {
	actions := []core.ActionDesc{}

	models, err := listOpenAIModels(ctx, o.client)
	if err != nil {
		return nil
	}
	for _, name := range models {
		metadata := map[string]any{
			"model": map[string]any{
				"supports": map[string]any{
					"media":       true,
					"multiturn":   true,
					"systemRole":  true,
					"tools":       true,
					"toolChoice":  true,
					"constrained": true,
				},
			},
			"versions": []string{},
			"stage":    string(ai.ModelStageStable),
		}
		metadata["label"] = fmt.Sprintf("%s - %s", o.Provider, name)

		actions = append(actions, core.ActionDesc{
			Type:     core.ActionTypeModel,
			Name:     fmt.Sprintf("%s/%s", o.Provider, name),
			Key:      fmt.Sprintf("/%s/%s/%s", core.ActionTypeModel, o.Provider, name),
			Metadata: metadata,
		})
	}

	return actions
}

func (o *StationOpenAICompatible) ResolveAction(g *genkit.Genkit, atype core.ActionType, name string) error {
	switch atype {
	case core.ActionTypeModel:
		o.DefineModel(g, o.Provider, name, ai.ModelInfo{
			Label:    fmt.Sprintf("%s - %s", o.Provider, name),
			Stage:    ai.ModelStageStable,
			Versions: []string{},
			Supports: &Multimodal,
		})
	}

	return nil
}

func listOpenAIModels(ctx context.Context, client *openai.Client) ([]string, error) {
	models := []string{}
	iter := client.Models.ListAutoPaging(ctx)
	for iter.Next() {
		m := iter.Current()
		models = append(models, m.ID)
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}

	return models, nil
}