export namespace main {
	
	export class Stats {
	    total: number;
	    due_today: number;
	
	    static createFrom(source: any = {}) {
	        return new Stats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.due_today = source["due_today"];
	    }
	}
	export class WidgetConfig {
	    window_x: number;
	    window_y: number;
	    always_on_top: boolean;
	    auto_start: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WidgetConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.window_x = source["window_x"];
	        this.window_y = source["window_y"];
	        this.always_on_top = source["always_on_top"];
	        this.auto_start = source["auto_start"];
	    }
	}
	export class WordCard {
	    id: number;
	    text: string;
	    definition: string;
	    example: string;
	    box: number;
	
	    static createFrom(source: any = {}) {
	        return new WordCard(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.text = source["text"];
	        this.definition = source["definition"];
	        this.example = source["example"];
	        this.box = source["box"];
	    }
	}

}

