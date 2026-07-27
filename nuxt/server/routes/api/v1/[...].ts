import { createBffHandler } from "@yueli/nuxt-runtime/server";

export default createBffHandler({
  mountPath: "/api/v1",
  resolveTarget({ event }) {
    return identityBffTarget(oidcConfig(event).downstreamBase);
  },
  credential: {
    async resolve({ event }) {
      const cfg = oidcConfig(event);
      let headers = await sessionAuthHeaders(event);
      if (!headers.authorization) {
        headers = ["GET", "HEAD", "OPTIONS"].includes(event.method)
          ? await guestSessionAuthHeaders(event, cfg.clientId, false)
          : await guestSessionAuthHeaders(event, cfg.clientId);
      }
      return identityBffCredential(headers);
    },
  },
});
