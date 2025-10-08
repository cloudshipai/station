# Environment Copy Feature - Implementation Summary

## ✅ Completed Backend Implementation

### 1. Fixed Lighthouse Endpoint (Completed)
- Updated default endpoint from `lighthouse.cloudshipai.com:443` to `lighthouse.cloudshipai.com:50051`
- Files modified:
  - `internal/lighthouse/client.go:148`
  - `internal/config/config.go:65`

### 2. Environment Copy Service (Completed)
**File**: `internal/services/environment_copy_service.go`

**Features**:
- Copies MCP servers with file-based config generation
- Copies agents with .prompt file generation
- Detects name conflicts (UNIQUE constraint: name + environment_id)
- Preserves agent-tool relationships
- Regenerates template.json for target environment
- Returns detailed conflict and error information

**Key Methods**:
- `CopyEnvironment(sourceEnvID, targetEnvID)` - Main copy orchestrator
- `copyMCPServer()` - Copies individual MCP server with conflict detection
- `copyAgent()` - Copies agent with .prompt file generation
- `generateAgentPromptFile()` - Creates .prompt YAML file
- `regenerateTemplateJSON()` - Rebuilds target environment template.json

**Conflict Detection**:
```go
type CopyConflict struct {
    Type          string  // "mcp_server" or "agent"
    Name          string  // Conflicting item name
    Reason        string  // Human-readable explanation
    SourceID      int64   // ID in source environment
    ConflictingID *int64  // ID of existing item in target (if applicable)
}
```

### 3. API Endpoint (Completed)
**File**: `internal/api/v1/environments.go`

**Endpoint**: `POST /api/v1/environments/:env_id/copy`

**Request**:
```json
{
  "target_environment_id": 2
}
```

**Response (Success - 200)**:
```json
{
  "success": true,
  "source_environment": "prod",
  "target_environment": "staging",
  "mcp_servers_copied": 3,
  "agents_copied": 5,
  "conflicts": [],
  "errors": [],
  "message": "Copied 3 MCP servers and 5 agents. 0 conflicts detected. Run 'stn sync staging' to apply changes."
}
```

**Response (Partial Success - 206)**:
```json
{
  "success": false,
  "source_environment": "prod",
  "target_environment": "staging",
  "mcp_servers_copied": 2,
  "agents_copied": 3,
  "conflicts": [
    {
      "type": "mcp_server",
      "name": "aws-cost-explorer",
      "reason": "MCP server 'aws-cost-explorer' already exists in target environment",
      "source_id": 5,
      "conflicting_id": 12
    },
    {
      "type": "agent",
      "name": "cost-analyzer",
      "reason": "Agent 'cost-analyzer' already exists in target environment",
      "source_id": 8,
      "conflicting_id": 15
    }
  ],
  "errors": [],
  "message": "Copied 2 MCP servers and 3 agents. 2 conflicts detected. Run 'stn sync staging' to apply changes."
}
```

**Validations**:
- Source environment must exist (404)
- Target environment must exist (404)
- Cannot copy to same environment (400)

## 🚧 Remaining Frontend Implementation

### 4. UI Modal Component (TODO)
**File**: `ui/src/components/modals/CopyEnvironmentModal.tsx` (to create)

**Required Features**:
- Environment selector dropdown (show all environments except source)
- Preview of what will be copied (agent count, MCP server count)
- Conflict display with warnings
- Progress indicator during copy
- Success/error messaging
- "Sync Now" button after successful copy

**Props Interface**:
```typescript
interface CopyEnvironmentModalProps {
  isOpen: boolean;
  onClose: () => void;
  sourceEnvironmentId: number;
  sourceEnvironmentName: string;
  environments: Environment[];
  onCopyComplete: () => void;
}
```

**Component State**:
```typescript
const [selectedTargetEnv, setSelectedTargetEnv] = useState<number | null>(null);
const [copying, setCopying] = useState(false);
const [result, setResult] = useState<CopyResult | null>(null);
const [showConflicts, setShowConflicts] = useState(false);
```

**API Integration**:
```typescript
const handleCopy = async () => {
  setCopying(true);
  try {
    const response = await apiClient.post(
      `/environments/${sourceEnvironmentId}/copy`,
      { target_environment_id: selectedTargetEnv }
    );
    setResult(response.data);
  } catch (error) {
    // Handle error
  } finally {
    setCopying(false);
  }
};
```

### 5. Copy Button in EnvironmentNode (TODO)
**File**: `ui/src/App.tsx` (line 1243)

**Changes Required**:

1. **Add Copy icon import** (line 20):
```typescript
import { Bot, Server, Layers, MessageSquare, Users, Package, Ship, CircleCheck, Globe, Database, Edit, Eye, ArrowLeft, Save, X, Play, Plus, Archive, Trash2, Settings, Link, Download, FileText, AlertTriangle, ChevronDown, ChevronRight, Rocket, Copy } from 'lucide-react';
```

2. **Add copy handler to EnvironmentNode**:
```typescript
const EnvironmentNode = ({ data }: NodeProps) => {
  const handleDeleteClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (data.onDeleteEnvironment && data.environmentId) {
      data.onDeleteEnvironment(data.environmentId, data.label);
    }
  };

  const handleCopyClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (data.onCopyEnvironment && data.environmentId) {
      data.onCopyEnvironment(data.environmentId, data.label);
    }
  };

  return (
    <div className="w-[320px] h-[160px] px-4 py-3 shadow-lg border border-tokyo-orange rounded-lg relative bg-tokyo-dark2 group">
      <Handle type="source" position={Position.Right} style={{ background: '#ff9e64', width: 12, height: 12 }} />
      <Handle type="source" position={Position.Bottom} style={{ background: '#7dcfff', width: 12, height: 12 }} />

      {/* Action buttons - appears on hover */}
      <div className="absolute top-2 right-2 flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity duration-200">
        {/* Copy button */}
        <button
          onClick={handleCopyClick}
          className="p-1 rounded bg-tokyo-blue hover:bg-blue-600 text-tokyo-bg"
          title={`Copy environment "${data.label}"`}
        >
          <Copy className="h-3 w-3" />
        </button>

        {/* Delete button - only for non-default environments */}
        {data.label !== 'default' && (
          <button
            onClick={handleDeleteClick}
            className="p-1 rounded bg-tokyo-red hover:bg-red-600 text-tokyo-bg"
            title={`Delete environment "${data.label}"`}
          >
            <Trash2 className="h-3 w-3" />
          </button>
        )}
      </div>

      <div className="flex items-center gap-2 mb-3">
        <Globe className="h-6 w-6 text-tokyo-orange" />
        <div className="font-mono text-lg text-tokyo-orange font-bold">{data.label}</div>
      </div>
      <div className="text-sm text-tokyo-comment mb-3">{data.description}</div>
      <div className="flex gap-4 text-sm font-mono">
        <div>
          <span className="text-tokyo-blue">{data.agentCount}</span>
          <span className="text-tokyo-comment"> agents</span>
        </div>
        <div>
          <span className="text-tokyo-cyan">{data.serverCount}</span>
          <span className="text-tokyo-comment"> servers</span>
        </div>
      </div>
    </div>
  );
};
```

3. **Add modal state to EnvironmentsPage** (line 1315):
```typescript
const EnvironmentsPage = () => {
  // ... existing state ...
  const [isCopyModalOpen, setIsCopyModalOpen] = useState(false);
  const [copySourceEnvId, setCopySourceEnvId] = useState<number | null>(null);
  const [copySourceEnvName, setCopySourceEnvName] = useState<string>('');

  // ... existing handlers ...

  const handleCopyEnvironment = (environmentId: number, environmentName: string) => {
    setCopySourceEnvId(environmentId);
    setCopySourceEnvName(environmentName);
    setIsCopyModalOpen(true);
  };

  const handleCopyComplete = () => {
    // Refresh environments and graph
    setRebuildingGraph(true);
    // Refetch environments
  };

  // ... in environment node creation ...
  newNodes.push({
    id: `env-${selectedEnvironment}`,
    type: 'environment',
    position: { x: 0, y: 0 },
    data: {
      label: selectedEnv?.name || 'Environment',
      description: selectedEnv?.description || 'Environment Hub',
      agentCount: agents.length,
      serverCount: mcpServers.length,
      environmentId: selectedEnvironment,
      onDeleteEnvironment: handleDeleteEnvironment,
      onCopyEnvironment: handleCopyEnvironment,  // ADD THIS
    },
  });

  // ... at end of return statement, add modal ...
  return (
    <div>
      {/* ... existing JSX ... */}

      <CopyEnvironmentModal
        isOpen={isCopyModalOpen}
        onClose={() => setIsCopyModalOpen(false)}
        sourceEnvironmentId={copySourceEnvId}
        sourceEnvironmentName={copySourceEnvName}
        environments={environments}
        onCopyComplete={handleCopyComplete}
      />
    </div>
  );
};
```

4. **Import the modal component** (line 37):
```typescript
import { CopyEnvironmentModal } from './components/modals/CopyEnvironmentModal';
```

## 📋 Testing Checklist

### Backend Testing (with stn up container)

1. **Setup Test Environments**:
```bash
# Inside container
docker exec -it station-server bash

# Create test environments
curl -X POST http://localhost:8585/api/v1/environments \
  -H "Content-Type: application/json" \
  -d '{"name": "test-source", "description": "Source environment for testing"}'

curl -X POST http://localhost:8585/api/v1/environments \
  -H "Content-Type: application/json" \
  -d '{"name": "test-target", "description": "Target environment for testing"}'
```

2. **Create Test Data in Source Environment**:
- Create 2-3 MCP servers via API or file config
- Create 2-3 agents via API or .prompt files
- Run `stn sync test-source` to ensure everything is synced

3. **Test Copy Operation**:
```bash
# Get environment IDs
curl http://localhost:8585/api/v1/environments

# Copy from source to target (replace IDs)
curl -X POST http://localhost:8585/api/v1/environments/1/copy \
  -H "Content-Type: application/json" \
  -d '{"target_environment_id": 2}'

# Verify response shows copied items
```

4. **Verify File Generation**:
```bash
# Check target environment files were created
ls -la ~/.config/station/environments/test-target/agents/
cat ~/.config/station/environments/test-target/template.json
```

5. **Test Conflict Detection**:
```bash
# Copy again (should show conflicts)
curl -X POST http://localhost:8585/api/v1/environments/1/copy \
  -H "Content-Type: application/json" \
  -d '{"target_environment_id": 2}'

# Verify response shows conflicts array populated
```

6. **Run Sync**:
```bash
stn sync test-target
# Verify agents and MCP servers are discovered and connected
```

### Frontend Testing (TODO - after UI implementation)

1. **UI Flow Test**:
- Navigate to Environments page
- Hover over environment card
- Click copy button
- Select target environment from dropdown
- Review preview (agent count, MCP server count)
- Click "Copy Environment"
- Verify progress indicator shows
- Verify success message appears
- Verify conflicts are displayed (if any)
- Click "Sync Now" button
- Verify sync modal opens

2. **Edge Cases**:
- Try to copy to same environment (should be disabled/error)
- Try to copy when no other environments exist
- Test with empty source environment
- Test with conflicts

## 🔧 Integration Test Script

Create `dev-workspace/test-environment-copy.sh`:

```bash
#!/bin/bash
set -e

echo "🧪 Testing Environment Copy Feature"

# Get container name
CONTAINER="station-server"

echo "\n📦 1. Creating test environments..."
docker exec $CONTAINER curl -X POST http://localhost:8585/api/v1/environments \
  -H "Content-Type: application/json" \
  -d '{"name": "copy-test-source", "description": "Source for copy testing"}' | jq '.'

docker exec $CONTAINER curl -X POST http://localhost:8585/api/v1/environments \
  -H "Content-Type: application/json" \
  -d '{"name": "copy-test-target", "description": "Target for copy testing"}' | jq '.'

echo "\n📝 2. Getting environment IDs..."
ENVS=$(docker exec $CONTAINER curl -s http://localhost:8585/api/v1/environments | jq '.environments')
SOURCE_ID=$(echo $ENVS | jq '.[] | select(.name == "copy-test-source") | .id')
TARGET_ID=$(echo $ENVS | jq '.[] | select(.name == "copy-test-target") | .id')

echo "Source ID: $SOURCE_ID"
echo "Target ID: $TARGET_ID"

echo "\n🤖 3. Creating test agent in source environment..."
docker exec $CONTAINER curl -X POST http://localhost:8585/api/v1/agents \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"test-agent-1\",
    \"description\": \"Test agent for copy\",
    \"prompt\": \"You are a test agent\",
    \"model\": \"gpt-4o-mini\",
    \"max_steps\": 5,
    \"environment_id\": $SOURCE_ID
  }" | jq '.'

echo "\n📋 4. Listing source environment agents..."
docker exec $CONTAINER curl -s "http://localhost:8585/api/v1/agents?environment_id=$SOURCE_ID" | jq '.'

echo "\n🚀 5. Copying environment..."
COPY_RESULT=$(docker exec $CONTAINER curl -s -X POST "http://localhost:8585/api/v1/environments/$SOURCE_ID/copy" \
  -H "Content-Type: application/json" \
  -d "{\"target_environment_id\": $TARGET_ID}")

echo $COPY_RESULT | jq '.'

echo "\n✅ 6. Verifying copy results..."
AGENTS_COPIED=$(echo $COPY_RESULT | jq '.agents_copied')
CONFLICTS=$(echo $COPY_RESULT | jq '.conflicts | length')

if [ "$AGENTS_COPIED" -gt 0 ]; then
  echo "✓ Successfully copied $AGENTS_COPIED agents"
else
  echo "✗ Failed to copy agents"
  exit 1
fi

echo "\n🔄 7. Testing conflict detection (copy again)..."
CONFLICT_RESULT=$(docker exec $CONTAINER curl -s -X POST "http://localhost:8585/api/v1/environments/$SOURCE_ID/copy" \
  -H "Content-Type: application/json" \
  -d "{\"target_environment_id\": $TARGET_ID}")

echo $CONFLICT_RESULT | jq '.'

CONFLICTS=$(echo $CONFLICT_RESULT | jq '.conflicts | length')
if [ "$CONFLICTS" -gt 0 ]; then
  echo "✓ Conflict detection working ($CONFLICTS conflicts detected)"
else
  echo "✗ Conflict detection failed"
  exit 1
fi

echo "\n🎉 All tests passed!"
```

Make executable:
```bash
chmod +x dev-workspace/test-environment-copy.sh
```

## 📝 Documentation Updates Needed

1. **User Guide**: Add section on copying environments
2. **API Documentation**: Document POST /api/v1/environments/:id/copy endpoint
3. **CLI Documentation**: Mention that sync is required after copying

## 🎯 Next Steps

1. Implement `CopyEnvironmentModal.tsx` component
2. Add copy button and handlers to `App.tsx`
3. Run integration tests with `stn up` container
4. Test UI flow end-to-end
5. Document the feature

## 🐛 Known Limitations

1. **Tool Assignment**: Tools are only copied if matching tool name exists in target environment
2. **No Batch Operations**: Can only copy one environment at a time
3. **No Selective Copy**: Copies all agents and MCP servers (no filtering)
4. **No Merge Strategy**: Conflicts are skipped, not merged or renamed
5. **CloudShip Integration**: Only copies to local environments, not CloudShip-managed stations

## 🚀 Future Enhancements

1. **Selective Copy**: Allow users to choose specific agents/MCP servers to copy
2. **Conflict Resolution**: Offer merge, rename, or overwrite options
3. **Batch Copy**: Copy to multiple target environments at once
4. **Copy History**: Track copy operations in database
5. **Undo/Rollback**: Ability to undo a copy operation
6. **Variable Mapping**: Map/transform template variables during copy
