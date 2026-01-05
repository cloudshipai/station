import { Container, getContainer } from "@cloudflare/containers";

export class StationContainer extends Container {
  defaultPort = 8587;
  sleepAfter = "{{.CloudflareSleepAfter}}";
}

export default {
  async fetch(request, env) {
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
    
    const container = getContainer(env.STATION, "default");
    return container.fetch(request);
  }
};
