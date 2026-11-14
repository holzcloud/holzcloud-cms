## Registro del server

Quando viene aperta una pagina, il server scrive una riga nel suo registro: momento, indirizzo richiesto, nonché stato e dimensione della risposta. Il Suo indirizzo IP viene abbreviato prima della scrittura, per IPv4 alla rete /{ipv4} e per IPv6 alla rete /{ipv6}: il resto dell’indirizzo viene azzerato e non viene memorizzato. Il registro viene conservato per {days} giorni. La cancellazione avviene giorno per giorno, perché il giornale di sistema inizia ogni giorno un nuovo file e rimuove solo file interi. Lo scopo è la gestione e la sicurezza del sito web.

Se qualcuno sbaglia l’accesso all’area di amministrazione sotto /admin o se un accesso tramite single sign-on viene rifiutato, il server scrive l’indirizzo IP completo nel registro di sistema. Esso viene conservato per lo stesso periodo e riguarda solo le persone che lavorano a questo sito web.
