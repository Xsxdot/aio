/**
 * Config key utilities for parsing and building config keys with environment suffixes
 */

// Valid environment suffixes (only these are treated as environments)
const VALID_ENV_SUFFIXES = new Set(['dev', 'test', 'staging', 'prod']);

/**
 * Parse a config key to extract baseKey and environment
 * Rules:
 * - Only 'dev', 'test', 'staging', 'prod' are treated as environment suffixes
 * - If the last segment is one of these, it's treated as the environment
 * - Otherwise, the entire key is the baseKey and environment is 'global'
 * 
 * Examples:
 * - 'a.b.key' -> { baseKey: 'a.b.key', env: 'global' }
 * - 'a.b.key.dev' -> { baseKey: 'a.b.key', env: 'dev' }
 * - 'system.global' -> { baseKey: 'system.global', env: 'global' }
 * - 'app.cert.prod' -> { baseKey: 'app.cert', env: 'prod' }
 */
export function parseKeyEnv(key: string): { baseKey: string; env: string } {
  if (!key) return { baseKey: '', env: 'global' };
  
  const parts = key.split('.');
  if (parts.length < 2) return { baseKey: key, env: 'global' };
  
  const lastPart = parts[parts.length - 1];
  
  // Check if the last segment is a valid environment suffix
  if (VALID_ENV_SUFFIXES.has(lastPart)) {
    return {
      baseKey: parts.slice(0, -1).join('.'),
      env: lastPart,
    };
  }
  
  // Not an environment suffix, entire key is the baseKey
  return { baseKey: key, env: 'global' };
}

/**
 * Build a full config key from baseKey and environment
 * Rules:
 * - If env is 'global', return just the baseKey (no suffix)
 * - Otherwise, return baseKey.env
 * 
 * Examples:
 * - buildConfigKey('a.b.key', 'global') -> 'a.b.key'
 * - buildConfigKey('a.b.key', 'dev') -> 'a.b.key.dev'
 * - buildConfigKey('app.cert', 'prod') -> 'app.cert.prod'
 */
export function buildConfigKey(baseKey: string, env: string): string {
  if (!baseKey) return '';
  if (env === 'global') return baseKey;
  return `${baseKey}.${env}`;
}

/**
 * Get all valid environment names including 'global'
 */
export function getAllEnvironments(): string[] {
  return ['dev', 'test', 'staging', 'prod', 'global'];
}

/**
 * Check if a string is a valid environment suffix (not including 'global')
 */
export function isValidEnvSuffix(env: string): boolean {
  return VALID_ENV_SUFFIXES.has(env);
}


