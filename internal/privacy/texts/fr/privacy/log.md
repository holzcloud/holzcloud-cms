## Journal du serveur

Lors de l’appel d’une page, le serveur inscrit une ligne dans son journal : l’heure, l’adresse appelée ainsi que le statut et la taille de la réponse. Votre adresse IP est tronquée avant l’inscription, au réseau /{ipv4} pour IPv4 et au réseau /{ipv6} pour IPv6 : le reste de l’adresse est mis à zéro et n’est pas enregistré. Le journal est conservé {days} jours. Il est effacé jour par jour, car le journal système commence un nouveau fichier chaque jour et ne supprime que des fichiers entiers. La finalité est l’exploitation et la sécurité du site.

Si quelqu’un échoue à se connecter à l’espace d’administration sous /admin, ou si une connexion par authentification unique (single sign-on) est refusée, le serveur inscrit l’adresse IP complète dans le journal système. Elle est conservée pendant la même durée et ne concerne que les personnes qui travaillent sur ce site.
