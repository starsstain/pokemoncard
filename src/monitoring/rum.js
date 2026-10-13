import { datadogRum } from "@datadog/browser-rum";
import { reactPlugin } from "@datadog/browser-rum-react";

const applicationId = import.meta.env.VITE_DD_APPLICATION_ID;
const clientToken = import.meta.env.VITE_DD_CLIENT_TOKEN;

// Browser RUM only accepts a client token. Never expose Datadog API/App keys here.
if (applicationId && clientToken) {
  datadogRum.init({
    applicationId,
    clientToken,
    site: import.meta.env.VITE_DD_SITE || "us5.datadoghq.com",
    service: "pokemon-battle-frontend",
    env: import.meta.env.VITE_DD_ENV || "school",
    version: import.meta.env.VITE_APP_VERSION || "1.0.0",
    sessionSampleRate: 100,
    sessionReplaySampleRate: 20,
    trackUserInteractions: true,
    trackResources: true,
    trackLongTasks: true,
    plugins: [reactPlugin({ router: false })],
    allowedTracingUrls: [
      { match: "http://127.0.0.1:8080", propagatorTypes: ["tracecontext", "datadog"] },
    ],
    traceSampleRate: 100,
  });

  datadogRum.setGlobalContextProperty("team", import.meta.env.VITE_DD_TEAM || "bj-xsolla-school-group-2");
}

export const isRumEnabled = Boolean(applicationId && clientToken);

export function reportRumError(error, context = {}) {
  if (!isRumEnabled) return;
  datadogRum.addError(error instanceof Error ? error : new Error(String(error)), context);
}

export function reportRumAction(name, context = {}) {
  if (!isRumEnabled) return;
  datadogRum.addAction(name, context);
}
