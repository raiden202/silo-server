import { useQuery } from "@tanstack/react-query";
import { api } from "@/api/client";
import { themeKeys } from "@/hooks/queries/keys";

const DEFAULT_SERVER_NAME = "Silo";
const DEFAULT_LOGIN_SUBTITLE = "Sign in with an existing account.";

interface ServerBranding {
  serverName: string;
  loginSubtitle: string;
}

/**
 * Fetches admin branding settings (server name, login subtitle).
 * Uses the public admin-css endpoint which already returns server settings
 * without authentication, so branding appears before login.
 */
export function useServerBranding(): ServerBranding {
  const { data } = useQuery({
    queryKey: [...themeKeys.all, "branding"] as const,
    queryFn: async () => {
      try {
        const result = await api<Record<string, string>>("/theme/branding");
        return result;
      } catch {
        return {};
      }
    },
    staleTime: 5 * 60_000,
  });

  return {
    serverName: data?.server_name || DEFAULT_SERVER_NAME,
    loginSubtitle: data?.login_subtitle || DEFAULT_LOGIN_SUBTITLE,
  };
}
