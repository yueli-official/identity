import { createUiPreset } from "@yueli/ui/theme";

// Account 只拥有自己的品牌色；中性色、圆角、阴影和图标契约由 Foundation 统一提供。
export default defineAppConfig(createUiPreset({ primary: "teal" }));
