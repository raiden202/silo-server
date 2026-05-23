export function buildLegacyWebhookSyncRedirectTarget(search: string): string {
  return search ? `/settings/webhook-sync${search}` : "/settings/webhook-sync";
}
