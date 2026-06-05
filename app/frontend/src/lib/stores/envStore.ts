export type AppEnv = 'production' | 'staging' | 'dev';

export const appEnv = (import.meta.env.PUBLIC_APP_ENV as AppEnv | undefined) ?? 'dev';

export const isStaging = appEnv === 'staging';
export const isProduction = appEnv === 'production';
