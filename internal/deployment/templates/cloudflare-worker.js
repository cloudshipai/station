import { Container } from "cloudflare:container";

export class StationContainer extends Container {
  defaultPort = 8587;
  sleepAfter = "{{.CloudflareSleepAfter}}";

  get envVars() {
    return {
      STATION_AI_PROVIDER: this.env.STATION_AI_PROVIDER || "{{.AIProvider}}",
      STATION_AI_MODEL: this.env.STATION_AI_MODEL || "{{.AIModel}}",
      STATION_AI_API_KEY: this.env.STATION_AI_API_KEY,
      STN_BUNDLE_ID: this.env.STN_BUNDLE_ID,
      STATION_MCP_POOLING: "true",
      STATION_MCP_PORT: "8587",
      STN_DEV_MODE: "false",
      STATION_ENCRYPTION_KEY: this.env.STATION_ENCRYPTION_KEY,
      {{range $key, $value := .EnvironmentVariables}}
      {{$key}}: this.env.{{$key}},
      {{end}}
    };
  }
}

export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);
    
    if (url.pathname === "/health" || url.pathname === "/healthz") {
      return new Response(JSON.stringify({ 
        status: "ok",
        service: "station-{{.EnvironmentName}}",
        timestamp: new Date().toISOString()
      }), {
        headers: { "Content-Type": "application/json" }
      });
    }
    
    if (url.pathname === "/_status") {
      try {
        const container = env.STATION.getByName("default");
        await container.fetch(new Request("http://internal/health"));
        return new Response(JSON.stringify({
          status: "running",
          container: "default",
          environment: "{{.EnvironmentName}}"
        }), {
          headers: { "Content-Type": "application/json" }
        });
      } catch (e) {
        return new Response(JSON.stringify({
          status: "error",
          error: e.message
        }), {
          status: 500,
          headers: { "Content-Type": "application/json" }
        });
      }
    }
    
    try {
      const container = env.STATION.getByName("default");
      return await container.fetch(request);
    } catch (e) {
      console.error("Container error:", e);
      return new Response(JSON.stringify({
        error: "Container unavailable",
        message: e.message,
        hint: "The container may be starting up. Please retry in a few seconds."
      }), {
        status: 503,
        headers: { 
          "Content-Type": "application/json",
          "Retry-After": "5"
        }
      });
    }
  }
};
