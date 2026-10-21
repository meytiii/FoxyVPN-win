export namespace core {
	
	export class VpnStatus {
	    state: string;
	    error_message?: string;
	    selected_country: string;
	    selected_city: string;
	    connected_node: string;
	    ping_ms: number;
	    upload_speed_bytes: number;
	    download_speed_bytes: number;
	    total_uploaded_bytes: number;
	    total_downloaded_bytes: number;
	    quota_limit_bytes: number;
	    quota_remaining_bytes: number;
	    quota_reset_epoch: number;
	    is_unlimited: boolean;
	    is_signed_in: boolean;
	    user_email: string;
	    tls_split_mode: number;
	    system_proxy_enabled: boolean;
	    socks_port: number;
	    http_port: number;
	    socks_bind_address: string;
	    real_ip: string;
	    real_country: string;
	    exit_ip: string;
	    exit_country: string;
	    is_protected: boolean;
	    exit_check_enabled: boolean;
	    dns_provider: string;
	    custom_doh_url: string;
	    bootstrap_dns: string;
	    custom_bootstrap_ip: string;
	    custom_edge_address: string;
	    upstream_proxy_enabled: boolean;
	    upstream_proxy_type: string;
	    upstream_proxy_host: string;
	    upstream_proxy_port: number;
	    minimize_to_tray: boolean;
	    close_to_tray: boolean;
	    launch_on_startup: boolean;
	    bypass_lan: boolean;
	    custom_bypass_list: string;
	
	    static createFrom(source: any = {}) {
	        return new VpnStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.error_message = source["error_message"];
	        this.selected_country = source["selected_country"];
	        this.selected_city = source["selected_city"];
	        this.connected_node = source["connected_node"];
	        this.ping_ms = source["ping_ms"];
	        this.upload_speed_bytes = source["upload_speed_bytes"];
	        this.download_speed_bytes = source["download_speed_bytes"];
	        this.total_uploaded_bytes = source["total_uploaded_bytes"];
	        this.total_downloaded_bytes = source["total_downloaded_bytes"];
	        this.quota_limit_bytes = source["quota_limit_bytes"];
	        this.quota_remaining_bytes = source["quota_remaining_bytes"];
	        this.quota_reset_epoch = source["quota_reset_epoch"];
	        this.is_unlimited = source["is_unlimited"];
	        this.is_signed_in = source["is_signed_in"];
	        this.user_email = source["user_email"];
	        this.tls_split_mode = source["tls_split_mode"];
	        this.system_proxy_enabled = source["system_proxy_enabled"];
	        this.socks_port = source["socks_port"];
	        this.http_port = source["http_port"];
	        this.socks_bind_address = source["socks_bind_address"];
	        this.real_ip = source["real_ip"];
	        this.real_country = source["real_country"];
	        this.exit_ip = source["exit_ip"];
	        this.exit_country = source["exit_country"];
	        this.is_protected = source["is_protected"];
	        this.exit_check_enabled = source["exit_check_enabled"];
	        this.dns_provider = source["dns_provider"];
	        this.custom_doh_url = source["custom_doh_url"];
	        this.bootstrap_dns = source["bootstrap_dns"];
	        this.custom_bootstrap_ip = source["custom_bootstrap_ip"];
	        this.custom_edge_address = source["custom_edge_address"];
	        this.upstream_proxy_enabled = source["upstream_proxy_enabled"];
	        this.upstream_proxy_type = source["upstream_proxy_type"];
	        this.upstream_proxy_host = source["upstream_proxy_host"];
	        this.upstream_proxy_port = source["upstream_proxy_port"];
	        this.minimize_to_tray = source["minimize_to_tray"];
	        this.close_to_tray = source["close_to_tray"];
	        this.launch_on_startup = source["launch_on_startup"];
	        this.bypass_lan = source["bypass_lan"];
	        this.custom_bypass_list = source["custom_bypass_list"];
	    }
	}

}

export namespace exitcheck {
	
	export class IpLocationInfo {
	    ip: string;
	    country_code: string;
	    is_protected: boolean;
	
	    static createFrom(source: any = {}) {
	        return new IpLocationInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ip = source["ip"];
	        this.country_code = source["country_code"];
	        this.is_protected = source["is_protected"];
	    }
	}

}

export namespace logger {
	
	export class LogEntry {
	    id: number;
	    timestamp_ms: number;
	    time_str: string;
	    level: string;
	    tag: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.timestamp_ms = source["timestamp_ms"];
	        this.time_str = source["time_str"];
	        this.level = source["level"];
	        this.tag = source["tag"];
	        this.message = source["message"];
	    }
	}

}

export namespace remotesettings {
	
	export class VpnProtocol {
	    name: string;
	    host: string;
	    port: number;
	    scheme: string;
	    templateString: string;
	
	    static createFrom(source: any = {}) {
	        return new VpnProtocol(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.scheme = source["scheme"];
	        this.templateString = source["templateString"];
	    }
	}
	export class VpnServerNode {
	    hostname: string;
	    port: number;
	    quarantined: boolean;
	    protocols: VpnProtocol[];
	
	    static createFrom(source: any = {}) {
	        return new VpnServerNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hostname = source["hostname"];
	        this.port = source["port"];
	        this.quarantined = source["quarantined"];
	        this.protocols = this.convertValues(source["protocols"], VpnProtocol);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class VpnCity {
	    name: string;
	    code: string;
	    servers: VpnServerNode[];
	
	    static createFrom(source: any = {}) {
	        return new VpnCity(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.code = source["code"];
	        this.servers = this.convertValues(source["servers"], VpnServerNode);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class VpnCountry {
	    name: string;
	    code: string;
	    cities: VpnCity[];
	
	    static createFrom(source: any = {}) {
	        return new VpnCountry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.code = source["code"];
	        this.cities = this.convertValues(source["cities"], VpnCity);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}

export namespace settings {
	
	export class AppSettings {
	    exit_check_enabled: boolean;
	    dns_provider: string;
	    custom_doh_url: string;
	    bootstrap_dns_provider: string;
	    custom_bootstrap_ip: string;
	    socks_port: number;
	    socks_bind_address: string;
	    http_proxy_enabled: boolean;
	    http_proxy_port: number;
	    proxy_only_mode: boolean;
	    custom_edge_address: string;
	    tls_split_mode: number;
	    upstream_proxy_enabled: boolean;
	    upstream_proxy_type: string;
	    upstream_proxy_host: string;
	    upstream_proxy_port: number;
	    upstream_proxy_username: string;
	    upstream_proxy_password: string;
	    minimize_to_tray: boolean;
	    close_to_tray: boolean;
	    launch_on_startup: boolean;
	    bypass_lan: boolean;
	    custom_bypass_list: string;
	
	    static createFrom(source: any = {}) {
	        return new AppSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.exit_check_enabled = source["exit_check_enabled"];
	        this.dns_provider = source["dns_provider"];
	        this.custom_doh_url = source["custom_doh_url"];
	        this.bootstrap_dns_provider = source["bootstrap_dns_provider"];
	        this.custom_bootstrap_ip = source["custom_bootstrap_ip"];
	        this.socks_port = source["socks_port"];
	        this.socks_bind_address = source["socks_bind_address"];
	        this.http_proxy_enabled = source["http_proxy_enabled"];
	        this.http_proxy_port = source["http_proxy_port"];
	        this.proxy_only_mode = source["proxy_only_mode"];
	        this.custom_edge_address = source["custom_edge_address"];
	        this.tls_split_mode = source["tls_split_mode"];
	        this.upstream_proxy_enabled = source["upstream_proxy_enabled"];
	        this.upstream_proxy_type = source["upstream_proxy_type"];
	        this.upstream_proxy_host = source["upstream_proxy_host"];
	        this.upstream_proxy_port = source["upstream_proxy_port"];
	        this.upstream_proxy_username = source["upstream_proxy_username"];
	        this.upstream_proxy_password = source["upstream_proxy_password"];
	        this.minimize_to_tray = source["minimize_to_tray"];
	        this.close_to_tray = source["close_to_tray"];
	        this.launch_on_startup = source["launch_on_startup"];
	        this.bypass_lan = source["bypass_lan"];
	        this.custom_bypass_list = source["custom_bypass_list"];
	    }
	}

}

