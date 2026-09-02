import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

const page = readFileSync(
  new URL("../app/pages/admin/assets.vue", import.meta.url),
  "utf8",
);
const layout = readFileSync(
  new URL("../app/layouts/admin.vue", import.meta.url),
  "utf8",
);
const registrations = readFileSync(
  new URL("../app/components/AssetRegistrationsPanel.vue", import.meta.url),
  "utf8",
);
const library = readFileSync(
  new URL("../app/components/AssetLibraryPanel.vue", import.meta.url),
  "utf8",
);
const styles = readFileSync(
  new URL("../app/assets/css/main.css", import.meta.url),
  "utf8",
);

describe("Asset management surface", () => {
  it("uses the shared page header and one tabbed task surface", () => {
    expect(page).toContain(
      '<PageHeader title="资源管理" icon="i-tabler-photo-cog">',
    );
    expect(page).toContain("<TabbedSurface");
    expect(page).toContain("data-asset-control-surface");
    expect(page).not.toContain("lg:grid-cols-6");
    expect(page).not.toContain("item.count");
  });

  it("keeps loading and failure feedback in the affected surface", () => {
    expect(page).toContain("const sectionIssues = reactive");
    expect(page).toContain("const currentSectionIssue = computed");
    expect(page).toContain('title="资源数据加载失败"');
    expect(page).toContain('label="重新加载"');
    expect(page).not.toContain("toast.add");
  });

  it("uses the commercial three-layer admin shell", () => {
    expect(layout).toContain("<YAdminConsoleLayout");
    expect(layout).toContain("data-account-admin-shell");
    expect(layout).not.toContain("const adminShellUi");
    expect(layout).not.toContain("<UDashboardPanel");
    expect(styles).toContain('@import "@yueli/ui/tailwind.css";');
  });

  it("keeps revision and digest inside the low-frequency application history", () => {
    const header = registrations.match(/<header[\s\S]*?<\/header>/u)?.[0] || "";
    expect(header).not.toContain("Revision");
    expect(header).not.toContain("effectiveDigest");
    expect(registrations).toContain("申请记录");
    expect(registrations).toContain("application.revision");
  });

  it("renders public previews through the same-origin media facade", () => {
    expect(page).toContain("function assetPreviewURL(asset: AssetItem)");
    expect(page).toContain("`/media/${asset.mediaKey}?format=${encodeURIComponent(format)}&name=${encodeURIComponent(variant.key)}`");
    expect(library).toContain(':src="previewFor(asset)"');
    expect(library).not.toContain(':src="asset.cdnUrl"');
    expect(library).not.toContain("publicBaseUrl");
  });
});
