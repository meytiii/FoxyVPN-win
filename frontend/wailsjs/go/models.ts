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
	    is_signed_in: boolean;
	    tls_split_mode: number;
	    system_proxy_enabled: boolean;
	    socks_port: number;
	    http_port: number;
	
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
	        this.is_signed_in = source["is_signed_in"];
	        this.tls_split_mode = source["tls_split_mode"];
	        this.system_proxy_enabled = source["system_proxy_enabled"];
	        this.socks_port = source["socks_port"];
	        this.http_port = source["http_port"];
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

