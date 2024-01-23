(function () {
  "use strict";

  const parts = window.document.currentScript.src.split("/");
  const endpoint = parts[0] + "//" + parts[2] + "/p";

  function sendMetrics(eventType) {
    if (navigator.doNotTrack || document.visibilityState !== "visible") return;
    navigator.sendBeacon(endpoint, prepEvent(eventType));
  }

  const wpt = window.performance.timing;
  function prepEvent(eventType) {
    return JSON.stringify({
      n: eventType,
      l: window.location.href,
      r: document.referrer,
      lt: Math.max(0, wpt.loadEventEnd - wpt.navigationStart),
      fb: Math.max(0, wpt.responseStart - wpt.navigationStart),
      sw: screen.width,
      sh: screen.height,
      tz: Intl.DateTimeFormat().resolvedOptions().timeZone,
      pr: window.devicePixelRatio,
      pd: window.screen.pixelDepth,
    });
  }

  document.addEventListener("visibilitychange", () => {
    sendMetrics(document.visibilityState);
  });
  window.addEventListener("popstate", () => sendMetrics("popstate"));
  window.addEventListener("hashchange", () => sendMetrics("hashchange"));
  window.addEventListener("load", () => {
    sendMetrics("load");
    setInterval(() => { sendMetrics("ping"); }, 5000);
  });

  // expose a global function to send metrics:
  // window.pico = function (eventName) { sendMetrics(eventName); };
})();
