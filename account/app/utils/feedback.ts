import {
  createFeedbackNotifier,
  type FeedbackTone,
} from "@yueli/ui/feedback";

export interface AccountToastInput {
  id?: string | number;
  title?: string;
  description?: string;
  color?: string;
  duration?: number;
  type?: "foreground" | "background";
  close?: boolean;
  icon?: string;
  [key: string]: unknown;
}

const feedbackTones = new Set<FeedbackTone>([
  "neutral",
  "success",
  "info",
  "warning",
  "error",
]);

export function createAccountNotifier<NativeToastInput>(toast: {
  add(input: NativeToastInput): unknown;
}) {
  const notifier = createFeedbackNotifier(
    toast,
    (notice) =>
      ({
        ...notice,
        color: notice.tone,
        type: notice.foreground ? "foreground" : "background",
      }) as NativeToastInput,
  );
  return {
    add(input: AccountToastInput) {
      const tone = feedbackTones.has(input.color as FeedbackTone)
        ? (input.color as FeedbackTone)
        : "neutral";
      return notifier.add({
        id: input.id,
        title: input.title,
        description: input.description,
        tone,
        duration: input.duration,
        foreground:
          input.type === undefined ? undefined : input.type === "foreground",
        close: input.close,
        icon: input.icon,
      });
    },
  };
}
