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

