import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

describe("admin users mobile filters", () => {
  const source = readFileSync(
    new URL("../app/pages/admin/users.vue", import.meta.url),
    "utf8",
  );

  it("delegates the responsive filter panel to the shared collection module", () => {
    expect(source).toContain('filter-panel-title="筛选与排序"');
    expect(source).toContain("section: '筛选条件'");
    expect(source).toContain("section: '列表排序'");
    expect(source).toContain("ascendingLabel: '升序'");
    expect(source).toContain("descendingLabel: '降序'");
    expect(source).not.toContain("ascendingLabel: '切换");
    expect(source).not.toContain("descendingLabel: '切换");
    expect(source).not.toContain('v-model:open="filtersOpen"');
    expect(source).not.toContain("admin-users-collection");
    expect(source).not.toContain("<style scoped>");
  });

  it("uses the shared content-page header without duplicate metrics or prose", () => {
    expect(source).toContain(
      '<PageHeader title="用户管理" icon="i-tabler-users">',
    );
    expect(source).not.toContain("<template #subtitle>");
    expect(source).not.toContain("总用户</div>");
    expect(source).not.toContain("已封禁</div>");
  });

  it("keeps row actions touch friendly on mobile", () => {
    expect(source).toContain(
      'class="min-h-11 min-w-11 touch-manipulation sm:min-h-0 sm:min-w-0"',
    );
  });
});
