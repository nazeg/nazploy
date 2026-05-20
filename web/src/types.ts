export interface Site {
  id: string
  name: string
  domain: string
  port: number
  root_dir: string
  site_type: 'static' | 'proxy' | 'pocketbase'
  proxy_url?: string
  admin_email?: string
  admin_password?: string
  ssl_status: 'none' | 'pending' | 'active' | 'error'
  ssl_expiry?: string
  status: 'active' | 'paused'
  notes?: string
  created: string
  updated: string
}

export interface Database {
  id: string
  site_id: string
  name: string
  db_type: string
  port: number
  admin_email: string
  admin_password: string
  status: 'active' | 'paused'
  created: string
  updated: string
}

export interface Stats {
  total_sites: number
  active_sites: number
  ssl_active_count: number
  total_databases: number
  nginx_running: boolean
}

export interface CreateSiteRequest {
  name: string
  domain: string
  site_type: 'static' | 'proxy' | 'pocketbase'
  proxy_url?: string
  admin_email?: string
  notes?: string
}

export interface CreateDatabaseRequest {
  name: string
  admin_email: string
}
