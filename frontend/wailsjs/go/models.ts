export namespace models {
	
	export class InstallResult {
	    success: boolean;
	    appName: string;
	    version: string;
	    executable: string;
	    desktopFile: string;
	    symlinks: string[];
	    installedDir: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new InstallResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.appName = source["appName"];
	        this.version = source["version"];
	        this.executable = source["executable"];
	        this.desktopFile = source["desktopFile"];
	        this.symlinks = source["symlinks"];
	        this.installedDir = source["installedDir"];
	        this.message = source["message"];
	    }
	}
	export class InstalledApp {
	    name: string;
	    displayName: string;
	    version: string;
	    source: string;
	    sourceUrl?: string;
	    installDir?: string;
	    executablePath?: string;
	    desktopEntry?: string;
	    iconPath?: string;
	    // Go type: time
	    installedAt: any;
	    sizeMb: number;
	    symlinks?: string[];
	
	    static createFrom(source: any = {}) {
	        return new InstalledApp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.displayName = source["displayName"];
	        this.version = source["version"];
	        this.source = source["source"];
	        this.sourceUrl = source["sourceUrl"];
	        this.installDir = source["installDir"];
	        this.executablePath = source["executablePath"];
	        this.desktopEntry = source["desktopEntry"];
	        this.iconPath = source["iconPath"];
	        this.installedAt = this.convertValues(source["installedAt"], null);
	        this.sizeMb = source["sizeMb"];
	        this.symlinks = source["symlinks"];
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
	export class PackageResult {
	    name: string;
	    version: string;
	    description: string;
	    backend: string;
	    installed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PackageResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.backend = source["backend"];
	        this.installed = source["installed"];
	    }
	}
	export class SystemInfo {
	    distroId: string;
	    distroName: string;
	    distroVersion: string;
	    kernel: string;
	    arch: string;
	    nativeManager: string;
	    hasFlatpak: boolean;
	    hasSnap: boolean;
	    detectedManagers: string[];
	
	    static createFrom(source: any = {}) {
	        return new SystemInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.distroId = source["distroId"];
	        this.distroName = source["distroName"];
	        this.distroVersion = source["distroVersion"];
	        this.kernel = source["kernel"];
	        this.arch = source["arch"];
	        this.nativeManager = source["nativeManager"];
	        this.hasFlatpak = source["hasFlatpak"];
	        this.hasSnap = source["hasSnap"];
	        this.detectedManagers = source["detectedManagers"];
	    }
	}

}

