export interface ExternalLoginProvider {
  key: string;
  label: string;
  registrationPolicy: "verified_email" | "existing_user_only";
  enabled?: boolean;
}

export const externalLoginProviderUi: Record<
  string,
  { label: string; icon: string; clientIdLabel: string; secretLabel: string }
> = {
  google: {
    label: "Google",
    icon: "i-tabler-brand-google",
    clientIdLabel: "Client ID",
    secretLabel: "Client Secret",
  },
  qq: {
    label: "QQ",
    icon: "i-tabler-brand-qq",
    clientIdLabel: "APP ID",
    secretLabel: "APP Key",
  },
};

export function externalLoginProviderMeta(key: string) {
  return (
    externalLoginProviderUi[key] || {
      label: key.toUpperCase(),
      icon: "i-tabler-key",
      clientIdLabel: "Client ID",
      secretLabel: "Client Secret",
    }
  );
}
