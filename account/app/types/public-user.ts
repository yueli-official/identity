import type { MediaRef } from "~/utils/media";

export interface PublicSocialLink {
  label: string;
  url: string;
}

export interface PublicUser {
  userKey: string;
  handle: string;
  displayName: string;
  avatar?: MediaRef;
  cover?: MediaRef;
  bio: string;
  socialLinks: PublicSocialLink[];
}

export interface PublicUserResponse {
  user: PublicUser;
}
