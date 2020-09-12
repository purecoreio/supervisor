class HealthLog {

    public host: string;
    public logs: Array<any>;
    public emitter: any;

    public constructor(host: string, logs: Array<any>) {
        this.host = host;
        this.logs = logs;
        this.emitter = new EventEmitter();
    }

    public pushLog(log) {
        log = JSON.parse(log.toString('utf8'));
        this.emitter.emit('log', log);
        this.logs.push({
            time: Date.now(),
            log: log,
        })
        console.log(log);
        if (this.logs[0].time < Date.now() - 3600 * 24 * 1000) {
            // delete logs older than 24h
            delete this.logs[0];
        }
    }

}