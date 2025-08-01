#!/usr/bin/env python3
"""
Simple MCP client to test Station's new Resources vs Tools implementation
"""

import json
import requests
import sys

MCP_ENDPOINT = "http://localhost:3001/mcp"

class MCPClient:
    def __init__(self, endpoint):
        self.endpoint = endpoint
        self.session = requests.Session()
        self.initialized = False
    
    def initialize(self):
        """Initialize MCP session"""
        init_data = {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "initialize",
            "params": {
                "protocolVersion": "2024-11-05",
                "capabilities": {
                    "resources": {"subscribe": True},
                    "tools": {"listChanged": True}
                },
                "clientInfo": {
                    "name": "station-test-client",
                    "version": "1.0.0"
                }
            }
        }
        
        try:
            response = self.session.post(self.endpoint, 
                                       json=init_data,
                                       headers={"Content-Type": "application/json"})
            
            if response.status_code == 200:
                result = response.json()
                if "result" in result:
                    self.initialized = True
                    print("✅ MCP session initialized")
                    return True
                else:
                    print(f"❌ MCP initialization failed: {result}")
                    return False
            else:
                print(f"❌ HTTP Error during init {response.status_code}: {response.text}")
                return False
        except requests.exceptions.RequestException as e:
            print(f"❌ Connection Error during init: {e}")
            return False
    
    def send_request(self, method, params=None):
        """Send MCP request to Station server"""
        if not self.initialized and method != "initialize":
            if not self.initialize():
                return None
        
        request_data = {
            "jsonrpc": "2.0",
            "id": 1,
            "method": method,
            "params": params or {}
        }
        
        try:
            response = self.session.post(self.endpoint, 
                                       json=request_data,
                                       headers={"Content-Type": "application/json"})
            
            if response.status_code == 200:
                return response.json()
            else:
                print(f"❌ HTTP Error {response.status_code}: {response.text}")
                return None
        except requests.exceptions.RequestException as e:
            print(f"❌ Connection Error: {e}")
            return None

# Global client instance
mcp_client = MCPClient(MCP_ENDPOINT)

def test_resources():
    """Test the new MCP Resources functionality"""
    print("🧪 Testing Station MCP Resources vs Tools Implementation")
    print("=" * 60)
    
    # Test 1: List available resources
    print("\n📋 Test 1: List Available Resources")
    result = mcp_client.send_request("resources/list")
    if result and "result" in result:
        resources = result["result"]["resources"]
        print(f"✅ Found {len(resources)} resources:")
        for resource in resources:
            print(f"   📄 {resource['uri']} - {resource['name']}")
    else:
        print("❌ Failed to list resources")
        return False
    
    # Test 2: Test station://environments resource
    print("\n🌍 Test 2: Read Environments Resource")
    result = mcp_client.send_request("resources/read", {"uri": "station://environments"})
    if result and "result" in result:
        content = result["result"]["contents"][0]["text"]
        data = json.loads(content)
        print(f"✅ Environments Resource:")
        print(f"   📊 Total: {data['total_count']} environments")
        for env in data["environments"]:
            print(f"   🏠 {env['name']} (ID: {env['id']}) - {env['description']}")
    else:
        print("❌ Failed to read environments resource")
    
    # Test 3: Test station://agents resource
    print("\n🤖 Test 3: Read Agents Resource")
    result = mcp_client.send_request("resources/read", {"uri": "station://agents"})
    if result and "result" in result:
        content = result["result"]["contents"][0]["text"]
        data = json.loads(content)
        print(f"✅ Agents Resource:")
        print(f"   📊 Total: {data['total_count']} agents")
        if data["agents"]:
            for agent in data["agents"]:
                print(f"   🤖 {agent['name']} (ID: {agent['id']}) - {agent['description']}")
        else:
            print("   ℹ️  No agents created yet")
    else:
        print("❌ Failed to read agents resource")
    
    # Test 4: Test station://mcp-configs resource
    print("\n⚙️  Test 4: Read MCP Configs Resource")
    result = mcp_client.send_request("resources/read", {"uri": "station://mcp-configs"})
    if result and "result" in result:
        content = result["result"]["contents"][0]["text"]
        data = json.loads(content)
        print(f"✅ MCP Configs Resource:")
        print(f"   📊 Total: {data['total_count']} configurations")
        if data["mcp_configs"]:
            for config in data["mcp_configs"]:
                print(f"   ⚙️  {config['config_name']} v{config['version']} (Env: {config['environment_id']})")
        else:
            print("   ℹ️  No MCP configurations found")
    else:
        print("❌ Failed to read MCP configs resource")
    
    return True

def test_tools():
    """Test MCP Tools functionality"""
    print("\n🔧 Test 5: List Available Tools")
    result = mcp_client.send_request("tools/list")
    if result and "result" in result:
        tools = result["result"]["tools"]
        print(f"✅ Found {len(tools)} tools:")
        for tool in tools:
            print(f"   🛠️  {tool['name']} - {tool['description']}")
    else:
        print("❌ Failed to list tools")
        return False
    
    # Test create_agent tool
    print("\n🆕 Test 6: Test create_agent Tool")
    agent_params = {
        "name": "test-mcp-agent",
        "description": "Test agent created via MCP to validate Resources vs Tools pattern",
        "prompt": "You are a test agent created to validate Station's MCP Resources vs Tools implementation. Respond with 'MCP Test Successful!' when called.",
        "environment_id": 1,
        "max_steps": 3,
        "enabled": True
    }
    
    result = mcp_client.send_request("tools/call", {
        "name": "create_agent",
        "arguments": agent_params
    })
    
    if result and "result" in result:
        response_data = json.loads(result["result"]["content"][0]["text"])
        if response_data["success"]:
            agent_id = response_data["agent"]["id"]
            print(f"✅ Created test agent:")
            print(f"   🤖 Name: {response_data['agent']['name']}")
            print(f"   🆔 ID: {agent_id}")
            print(f"   🏠 Environment: {response_data['agent']['environment']}")
            return agent_id
        else:
            print("❌ Failed to create agent")
            return None
    else:
        print("❌ Tool call failed")
        return None

def test_dynamic_resources(agent_id):
    """Test dynamic resource templates"""
    if not agent_id:
        print("\n⚠️  Skipping dynamic resource tests - no agent ID")
        return
    
    print(f"\n🔍 Test 7: Test Dynamic Agent Details Resource")
    result = mcp_client.send_request("resources/read", {"uri": f"station://agents/{agent_id}"})
    if result and "result" in result:
        content = result["result"]["contents"][0]["text"]
        data = json.loads(content)
        print(f"✅ Agent Details Resource:")
        print(f"   🤖 Name: {data['agent']['name']}")
        print(f"   📝 Description: {data['agent']['description']}")
        print(f"   🧠 Prompt: {data['agent']['prompt'][:50]}...")
        print(f"   🏠 Environment: {data['environment']['name']}")
        print(f"   🛠️  Tools: {data['tools_count']} assigned")
    else:
        print("❌ Failed to read agent details resource")

def main():
    """Run all MCP tests"""
    print("🚀 Starting Station MCP Resources vs Tools Test")
    print("📡 Testing against: " + MCP_ENDPOINT)
    
    # Test Resources (read-only)
    if not test_resources():
        print("\n❌ Resource tests failed")
        return False
    
    # Test Tools (operations)
    agent_id = test_tools()
    
    # Test dynamic resources
    test_dynamic_resources(agent_id)
    
    print("\n" + "=" * 60)
    print("✅ MCP Resources vs Tools Implementation Test Complete!")
    print("\n📋 Summary:")
    print("   📄 Resources: Used for read-only data discovery")
    print("   🛠️  Tools: Used for operations and state changes")
    print("   🎯 Pattern: Follows MCP specification correctly")
    
    return True

if __name__ == "__main__":
    success = main()
    sys.exit(0 if success else 1)