export namespace backend {
	
	export class MotorConfig {
	    id: number;
	    name: string;
	    unit: string;
	    description: string;
	    dir: number;
	    speed: number;
	    resolution: number;
	    cwName: string;
	    ccwName: string;
	    mode: string;
	
	    static createFrom(source: any = {}) {
	        return new MotorConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.unit = source["unit"];
	        this.description = source["description"];
	        this.dir = source["dir"];
	        this.speed = source["speed"];
	        this.resolution = source["resolution"];
	        this.cwName = source["cwName"];
	        this.ccwName = source["ccwName"];
	        this.mode = source["mode"];
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

