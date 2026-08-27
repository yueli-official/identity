import { canonicalPageURL } from "../utils/canonical-origin";

export default defineEventHandler((event) => {
  const redirectURI = String(
    useRuntimeConfig(event).public.oidcRedirectUri || "",
  );
  const target = canonicalPageURL({
    requestURL: getRequestURL(event),
    redirectURI,
    method: event.method,
    accept: getRequestHeader(event, "accept") || "",
  });
  if (target) return sendRedirect(event, target, 307);
});
