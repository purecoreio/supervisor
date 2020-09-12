const http = require('http');
const https = require('https');
const socketio = require('socket.io');
const app = require('express')();

class SocketServer {

    public static io;
    public static authenticated = [];
    public static authenticatedHosts = [];

    public static healthEmitters: any = {};

    public getSocket(server) {
        return new socketio(server).on('connection', client => {
            client.on('authenticate', authInfo => {
                this.authenticate(client, authInfo);
            });
            client.on('health', extra => {
                if (SocketServer.getHost(client) != null && SocketServer.isAuthenticated(client)) {
                    try {
                        SocketServer.healthEmitters[client.id] = DockerLogger.getHealthEmitter(SocketServer.getHost(client).host.uuid)
                        if (SocketServer.healthEmitters[client.id] != null) {
                            SocketServer.healthEmitters[client.id].on('log', (log) => {
                                if (client.connected) {
                                    client.emit('healthLog', log);
                                } else {
                                    delete SocketServer.healthEmitters[client.id];
                                }
                            })
                        }
                    } catch (error) {
                        console.log(error)
                    }
                }
            });
            client.on('console', extra => {
                if (SocketServer.getHost(client) != null && SocketServer.isAuthenticated(client)) {
                    try {
                        DockerHelper.getContainer(SocketServer.getHost(client).host).then((container) => {
                            try {
                                DockerHelper.getLogStream(container).then((logStream) => {
                                    logStream.on('data', (data) => {
                                        if (!client.connected) {
                                            logStream.end();
                                        } else {
                                            try {
                                                client.emit('console', data.toString('utf-8').trim())
                                            } catch (error) {
                                                logStream.end();
                                            }
                                        }
                                    }).on('error', () => {
                                        logStream.end();
                                    })
                                })
                            } catch (error) {
                                // ignore
                            }
                        })
                    } catch (error) {
                        console.log(error)
                    }
                }
            })
            client.on('host', hostObject => {
                if (SocketServer.isMasterAuthenticated(client)) DockerHelper.createContainer(hostObject).catch((err) => { /*ignore*/ })
            });
            client.on('disconnect', () => { SocketServer.removeAuth(client.id) });
        })
    }

    public static getHost(client) {
        for (let index = 0; index < SocketServer.authenticatedHosts.length; index++) {
            const element = SocketServer.authenticatedHosts[index];
            if (element.client == client.id) return element.hostAuth; break;
        }
    }

    public static isMasterAuthenticated(client): boolean {
        return SocketServer.authenticated.includes(client.id);
    }

    public static isAuthenticated(client) {
        if (SocketServer.authenticated.includes(client.id)) {
            return true;
        } else {
            for (let index = 0; index < SocketServer.authenticatedHosts.length; index++) {
                const element = SocketServer.authenticatedHosts[index];
                if (element.client == client.id) return true; break;
            }
        }
    }

    public static addAuth(client, host?: any) {
        if (host == null) {
            if (!SocketServer.authenticated.includes(client.id)) {
                Supervisor.emitter.emit('clientConnected');
                this.authenticated.push(client.id);
            }
        } else {
            if (!SocketServer.authenticatedHosts.includes(client.id)) {
                Supervisor.emitter.emit('clientConnected');
                SocketServer.authenticatedHosts.push({ client: client.id, hostAuth: host });
            }
        }
        client.emit('authenticated')
    }

    public static removeAuth(clientid) {
        if (this.isAuthenticated(clientid)) {
            Supervisor.emitter.emit('clientDisconnected');
            SocketServer.authenticated = SocketServer.authenticated.filter(x => x !== clientid);
            SocketServer.authenticatedHosts = SocketServer.authenticatedHosts.filter(x => x.client !== clientid);
        }
    }

    public authenticate(client, authInfo) {
        if ((authInfo == null || authInfo == "" || authInfo == [] || authInfo == {})) {
            const accepetedHostnames = ["api.purecore.io", "purecore.io"]
            const hostname = client.handshake.headers.host.split(".").shift();
            if (accepetedHostnames.includes(hostname)) {
                SocketServer.addAuth(client)
            } else {
                client.disconnect()
            }
        } else if (typeof authInfo == "object") {
            if ('hash' in authInfo && authInfo.hash == Supervisor.machine.hash) {
                SocketServer.addAuth(client)
            } else if ('auth' in authInfo) {
                try {
                    let authHash = authInfo.auth;
                    let match = null;
                    for (let index = 0; index < Supervisor.hostAuths.length; index++) {
                        const element = Supervisor.hostAuths[index];
                        if (element.hash == authHash) {
                            match = element;
                            break;
                        }
                    }
                    if (match != null) {
                        SocketServer.addAuth(client, match)
                    } else {
                        client.disconnect()
                    }
                } catch (error) {
                    client.disconnect()
                }
            } else {
                client.disconnect()
            }
        } else {
            client.disconnect()
        }
    }

    public setup() {
        try {
            Supervisor.emitter.emit('creatingServer');
            const server = this.getHTTP();
            Supervisor.emitter.emit('createdServer');

            try {
                Supervisor.emitter.emit('creatingSocketServer');
                SocketServer.io = this.getSocket(server);
                server.listen(31518, () => {
                    Supervisor.emitter.emit('createdSocketServer');
                }).on('error', function (error) {
                    Supervisor.emitter.emit('errorCreatingSocketServer', new Error(error.code));
                });
            } catch (error) {
                Supervisor.emitter.emit('errorCreatingSocketServer', error);
            }
        } catch (error) {
            Supervisor.emitter.emit('errorCreatingServer', error);
        }
    }

    public getHTTP(): any {
        let httpServer = null;
        const cert = this.getCert();

        if (cert != null) {
            Supervisor.emitter.emit('certUse');
            httpServer = https.Server({
                key: cert.key,
                cert: cert.cert,
                ca: cert.ca
            }, app);
        } else {
            Supervisor.emitter.emit('certUnknown');
        }

        if (httpServer == null) {
            httpServer = http.createServer();
        }
        return httpServer;
    }

    public getCert(): Cert {
        Supervisor.emitter.emit('certLoading');
        const fs = require("fs");
        const letsencryptBase = "/etc/letsencrypt/"
        try {
            if (fs.existsSync(letsencryptBase) && fs.existsSync(letsencryptBase + "live/")) {
                let files = fs.readdirSync(letsencryptBase + "live/");
                let folders = [];
                files.forEach(file => {
                    if (fs.lstatSync(letsencryptBase + file).isDirectory()) folders.push(file);
                });
                if (folders != null && Array.isArray(folders) && folders.length > 0) {
                    Supervisor.emitter.emit('certFound');
                    return new Cert(fs.readFileSync(letsencryptBase + "live/" + folders[0] + "/privkey.pem"), fs.readFileSync(letsencryptBase + "live/" + folders[0] + "/cert.pem"), fs.readFileSync(letsencryptBase + "live/" + folders[0] + "/chain.pem"));
                } else {
                    Supervisor.emitter.emit('certNotSetup');
                    return null;
                }
            } else {
                Supervisor.emitter.emit('certNotInstalled');
                return null;
            }
        } catch (error) {
            Supervisor.emitter.emit('certReadingError');
            return null;
        }
    }

}