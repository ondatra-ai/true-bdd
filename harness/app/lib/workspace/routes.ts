/** Workspace navigation (mirrors tests/harness/helpers/ui.ts's `wsRoutes`). */
export const routes = {
  home: (sid: string) => `/sessions/${sid}/home`,
  architecture: (sid: string) => `/sessions/${sid}/architecture`,
  product: (sid: string) => `/sessions/${sid}/product`,
  features: (sid: string) => `/sessions/${sid}/product/features`,
  feature: (sid: string, id: string) => `/sessions/${sid}/product/features/${id}`,
  story: (sid: string, storyId: string) => `/sessions/${sid}/product/stories/${storyId}`,
  scenarios: (sid: string) => `/sessions/${sid}/product/scenarios`,
  builds: (sid: string) => `/sessions/${sid}/builds`,
};
