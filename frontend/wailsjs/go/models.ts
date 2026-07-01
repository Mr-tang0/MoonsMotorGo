export namespace backend {
	
	export class  {
	    name: string;
	    browser_download_url: string;
	
	    static createFrom(source: any = {}) {
	        return new (source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.browser_download_url = source["browser_download_url"];
	    }
	}
	export class GitHubRelease {
	    tag_name: string;
	    html_url: string;
	    assets: [];
	
	    static createFrom(source: any = {}) {
	        return new GitHubRelease(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tag_name = source["tag_name"];
	        this.html_url = source["html_url"];
	        this.assets = this.convertValues(source["assets"], );
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
	export class MotorConfig {
	    id: number;
	    name: string;
	    unit: string;
	    description: string;
	    speed: number;
	    resolution: number;
	    cwName: string;
	    ccwName: string;
	    mode: string;
	    newID: number;
	
	    static createFrom(source: any = {}) {
	        return new MotorConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.unit = source["unit"];
	        this.description = source["description"];
	        this.speed = source["speed"];
	        this.resolution = source["resolution"];
	        this.cwName = source["cwName"];
	        this.ccwName = source["ccwName"];
	        this.mode = source["mode"];
	        this.newID = source["newID"];
	    }
	}

}

export namespace main {
	
	export class APIResponse {
	    status: string;
	    message: string;
	    data: any;
	
	    static createFrom(source: any = {}) {
	        return new APIResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.message = source["message"];
	        this.data = source["data"];
	    }
	}

}

