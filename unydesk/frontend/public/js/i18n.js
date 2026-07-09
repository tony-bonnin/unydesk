(() => {
  const STORAGE_KEY = "unydesk.locale";
  const SUPPORTED_LOCALES = ["en", "fr", "de"];
  const LOCALE_LABELS = {
    en: "English",
    fr: "Français",
    de: "Deutsch",
  };

  const dictionaries = {
    en: {},
    fr: {
      "Sign In": "Connexion",
      "Go App · Alpine-friendly · API-first": "Application Go · Compatible Alpine · API-first",
      "TRINITY Labs Remote Access": "Accès distant TRINITY Labs",
      "Free Remote Access": "Accès distant gratuit",
      "A lightweight remote-control broker built in Go for Alpine Linux, musl-friendly deployment, and clean future integration with UnyPort. You can expose this instance through a generated UnyDesk ID or keep a direct connection model with IP and port.": "Un broker de contrôle distant léger, développé en Go pour Alpine Linux, compatible musl, avec une intégration future propre à UnyPort. Vous pouvez exposer cette instance via un ID UnyDesk généré ou conserver un modèle de connexion directe par IP et port.",
      "Client IP": "IP client",
      "Direct Address": "Adresse directe",
      "Runtime": "Exécution",
      "Version": "Version",
      "Server": "Serveur",
      "Loading…": "Chargement...",
      "Detecting...": "Détection...",
      "Detecting…": "Détection...",
      "UnyDesk ID": "ID UnyDesk",
      "Host Password": "Mot de passe du host",
      "Ephemeral Client Password": "Mot de passe client éphémère",
      "ID copied!": "ID copié !",
      "Password copied!": "Mot de passe copié !",
      "Open API Info": "Ouvrir les infos API",
      "Health Check": "État de santé",
      "Detecting system": "Détection du système",
      "Preparing suggestion…": "Préparation de la suggestion...",
      "Preparing suggestion...": "Préparation de la suggestion...",
      "Waiting for browser detection.": "En attente de la détection du navigateur.",
      "Detected in your browser. If needed, open the full download list.": "Détecté depuis votre navigateur. Si nécessaire, ouvrez la liste complète des téléchargements.",
      "All host downloads": "Tous les téléchargements host",
      "Windows builds include metadata and SHA256 checksums. SmartScreen warnings may still appear until code signing is enabled.": "Les builds Windows incluent des métadonnées et des sommes SHA256. Des alertes SmartScreen peuvent encore apparaître tant que la signature de code n’est pas activée.",
      "Windows builds include metadata and": "Les builds Windows incluent des métadonnées et des",
      "checksums. SmartScreen warnings may still appear until code signing is enabled.": "sommes de contrôle. Des alertes SmartScreen peuvent encore apparaître tant que la signature de code n’est pas activée.",
      "Quick host test:": "Test rapide du host :",
      "Connect to Host": "Se connecter au host",
      "Enter the remote UnyDesk ID and the password currently shown by the host binary on its local browser page.": "Saisissez l’ID UnyDesk distant et le mot de passe actuellement affiché par le binaire host sur sa page navigateur locale.",
      "Standalone Client": "Client autonome",
      "Open a direct viewer from this landing page with your local client ID and an ephemeral password.": "Ouvrez un viewer direct depuis cette page avec votre ID client local et un mot de passe éphémère.",
      "Client ID": "ID client",
      "Target Host ID": "ID du host cible",
      "Current password shown on the host": "Mot de passe actuellement affiché sur le host",
      "Connect to host": "Se connecter au host",
      "Open standalone client": "Ouvrir le client autonome",
      "Opening...": "Ouverture...",
      "No account required. Open a standalone client in a new tab.": "Aucun compte requis. Ouvrez un client autonome dans un nouvel onglet.",
      "Self-hosted remote access by TRINITY Edge Networks.": "Accès distant auto-hébergé par TRINITY Edge Networks.",
      "Next step: secure connect flow and real remote-control transport.": "Prochaine étape : flux de connexion sécurisé et transport de contrôle distant réel.",
      "Host Downloads": "Téléchargements host",
      "Manual platform selection stays available even when automatic detection is uncertain.": "La sélection manuelle de plateforme reste disponible si la détection automatique est incertaine.",
      "Close": "Fermer",
      "Download": "Télécharger",
      "ZIP": "ZIP",
      "Checksums": "Sommes de contrôle",
      "Open SHA256SUMS": "Ouvrir SHA256SUMS",
      "Load in modal": "Charger dans la fenêtre",
      "Checksums will appear here on demand.": "Les sommes de contrôle apparaîtront ici à la demande.",
      "Loading checksums...": "Chargement des sommes de contrôle...",
      "No checksums available.": "Aucune somme de contrôle disponible.",
      "Unable to load checksums right now.": "Impossible de charger les sommes de contrôle pour le moment.",
      "Accounts are managed locally through users.json. Public sign-up is disabled.": "Les comptes sont gérés localement via users.json. L’inscription publique est désactivée.",
      "Email": "E-mail",
      "Password": "Mot de passe",
      "Service Status": "État du service",
      "Inspect API and health endpoint responses without leaving the page.": "Inspectez les réponses API et santé sans quitter la page.",
      "Ready.": "Prêt.",
      "Choose an endpoint to inspect.": "Choisissez un endpoint à inspecter.",
      "Available": "Disponible",
      "Unavailable": "Indisponible",
      "Loading...": "Chargement...",
      "API Info": "Infos API",
      "Live response from the service health endpoint.": "Réponse en direct de l’endpoint santé du service.",
      "Live response from the API information endpoint.": "Réponse en direct de l’endpoint d’informations API.",
      "No response body.": "Aucun corps de réponse.",
      "The request failed before a response was received.": "La requête a échoué avant réception d’une réponse.",
      "Unable to open a standalone session right now.": "Impossible d’ouvrir une session autonome pour le moment.",
      "Enter a target host ID or hostname first.": "Saisissez d’abord un ID de host ou un nom d’hôte cible.",
      "Client identity is still loading.": "L’identité client est encore en cours de chargement.",
      "Enter the host password first.": "Saisissez d’abord le mot de passe du host.",
      "Authenticating host access...": "Authentification de l’accès host...",
      "Security token unavailable. Refresh the page and try again.": "Jeton de sécurité indisponible. Actualisez la page et réessayez.",
      "Client not installed on this device": "Client non installé sur cet appareil",
      "Unavailable": "Indisponible",
      "Detected locally from the installed host client.": "Détecté localement depuis le client host installé.",
      "Local host detected. Claim it once to bind this machine to your workspace.": "Host local détecté. Associez-le une fois pour rattacher cette machine à votre espace.",
      "Sign in to claim this local host and keep it attached to your workspace.": "Connectez-vous pour associer ce host local et le garder rattaché à votre espace.",
      "Claim local host": "Associer le host local",
      "Sign in to claim": "Se connecter pour associer",
      "Linking...": "Association...",
      "Linking the local host to your workspace...": "Association du host local à votre espace...",
      "Local host linked to your workspace.": "Host local associé à votre espace.",
      "Unable to claim the local host right now.": "Impossible d’associer le host local pour le moment.",
      "The public landing page only shows the host password when the local host client is installed on this device.": "La landing publique n’affiche le mot de passe du host que lorsque le client host local est installé sur cet appareil.",
      "Host password rotated locally.": "Mot de passe du host régénéré localement.",
      "Unable to rotate the local host password right now.": "Impossible de régénérer le mot de passe local du host pour le moment.",
      "Preparing standalone session...": "Préparation de la session autonome...",
      "Ephemeral client password regenerated.": "Mot de passe client éphémère régénéré.",
      "Recommended host: Windows 64-bits": "Host recommandé : Windows 64 bits",
      "Recommended host: Windows ARM64": "Host recommandé : Windows ARM64",
      "Recommended host: macOS Intel": "Host recommandé : macOS Intel",
      "Recommended host: macOS Apple Silicon": "Host recommandé : macOS Apple Silicon",
      "Recommended host: Linux 64-bits": "Host recommandé : Linux 64 bits",
      "Download Windows 64-bits": "Télécharger Windows 64 bits",
      "Download Windows ARM64": "Télécharger Windows ARM64",
      "Download macOS Intel": "Télécharger macOS Intel",
      "Download macOS Apple Silicon": "Télécharger macOS Apple Silicon",
      "Download Linux 64-bits": "Télécharger Linux 64 bits",
      "Android detected": "Android détecté",
      "Android host coming soon": "Host Android bientôt disponible",
      "Detected: Windows 64-bits.": "Détecté : Windows 64 bits.",
      "Detected: Windows on ARM.": "Détecté : Windows sur ARM.",
      "Detected: macOS Intel.": "Détecté : macOS Intel.",
      "Detected: macOS Apple Silicon.": "Détecté : macOS Apple Silicon.",
      "Detection was unclear. This is the fallback choice.": "La détection est incertaine. Ce choix est utilisé par défaut.",
      "Detected: Linux 64-bits. Distribution not exposed by this browser.": "Détecté : Linux 64 bits. Distribution non exposée par ce navigateur.",
      "Detected: Linux ARM64. Distribution not exposed by this browser.": "Détecté : Linux ARM64. Distribution non exposée par ce navigateur.",
      "Detected: Android. Native host APK is not published yet; use the standalone web client from this browser for now.": "Détecté : Android. L’APK host natif n’est pas encore publié ; utilisez le client web autonome depuis ce navigateur pour le moment.",
      "Host for Linux 64-bits": "Host pour Linux 64 bits",
      "Host for Linux ARM64": "Host pour Linux ARM64",
      "Host for Android": "Host pour Android",
      "Host for Windows 64-bits": "Host pour Windows 64 bits",
      "Host for Windows ARM64": "Host pour Windows ARM64",
      "Host for macOS Intel": "Host pour macOS Intel",
      "Host for macOS Apple Silicon": "Host pour macOS Apple Silicon",
      "Static binary for major x86_64 Linux distributions.": "Binaire statique pour les principales distributions Linux x86_64.",
      "Same host flow for ARM targets and lightweight edge nodes.": "Même flux host pour les cibles ARM et les nœuds edge légers.",
      "Android endpoint detected. Native APK is not published yet; use the standalone web client for now.": "Endpoint Android détecté. L’APK natif n’est pas encore publié ; utilisez le client web autonome pour le moment.",
      "Standard 64-bit Windows host binary for desktop and server editions.": "Binaire host Windows 64 bits standard pour éditions desktop et serveur.",
      "Windows on ARM build for newer ARM laptops and tablets.": "Build Windows on ARM pour laptops et tablettes ARM récents.",
      "Darwin build for Intel-based Mac systems.": "Build Darwin pour Mac Intel.",
      "Darwin build for Apple Silicon systems.": "Build Darwin pour Mac Apple Silicon.",
      "Coming soon": "Bientôt disponible",

      "Workspace": "Espace de travail",
      "Overview": "Vue d’ensemble",
      "Connect": "Connexion",
      "Machines": "Machines",
      "Sessions": "Sessions",
      "Account": "Compte",
      "User Settings": "Paramètres utilisateur",
      "Sign out": "Déconnexion",
      "Back to home": "Retour à l’accueil",
      "Remote Access Workspace": "Espace d’accès distant",
      "Connect with a one-time UnyDesk ID and password, then keep trusted machines registered for future access.": "Connectez-vous avec un ID UnyDesk et un mot de passe à usage ponctuel, puis enregistrez les machines de confiance pour les prochains accès.",
      "Loading session…": "Chargement de la session…",
      "Display name": "Nom affiché",
      "Authenticated user": "Utilisateur authentifié",
      "Browser Client ID": "ID client du navigateur",
      "Ready hosts": "Hosts prêts",
      "Registered Access": "Accès enregistrés",
      "Machines saved for repeat access stay visible even when offline.": "Les machines enregistrées restent visibles même hors ligne.",
      "Registered machines": "Machines enregistrées",
      "Not ready": "Non prêtes",
      "Connection Activity": "Activité de connexion",
      "Reusable session records for active and recent remote access.": "Historique des sessions réutilisables pour les accès distants actifs et récents.",
      "Live sessions": "Sessions actives",
      "Session records": "Sessions enregistrées",
      "Active, paused, and offline hosts from your saved fleet.": "Hosts actifs, en pause et hors ligne de votre parc enregistré.",
      "Loading registered machines…": "Chargement des machines enregistrées…",
      "Connect to a Host": "Se connecter à un host",
      "Enter the host ID and the password currently displayed by the host binary. After the first approved access, trusted machines can reconnect without retyping the password.": "Saisissez l’ID du host et le mot de passe actuellement affiché par le binaire host. Après le premier accès approuvé, les machines de confiance peuvent se reconnecter sans ressaisir le mot de passe.",
      "Host ID or registered machine": "ID du host ou machine enregistrée",
      "Connect or reuse access": "Se connecter ou réutiliser l’accès",
      "The first authenticated connection remembers this host for your account automatically.": "La première connexion authentifiée mémorise automatiquement ce host pour votre compte.",
      "Registered Machines": "Machines enregistrées",
      "Review saved machines, access state, platform, and last heartbeat before opening a session.": "Consultez les machines enregistrées, leur état d’accès, leur plateforme et leur dernier heartbeat avant d’ouvrir une session.",
      "Machine": "Machine",
      "Platform": "Plateforme",
      "Status": "Statut",
      "Action": "Action",
      "Loading hosts…": "Chargement des hosts…",
      "Session Manager": "Gestionnaire de sessions",
      "Monitor pending, active, and historical remote connections from one place.": "Surveillez les connexions distantes en attente, actives et passées depuis un seul endroit.",
      "Session": "Session",
      "Target": "Cible",
      "Viewer": "Viewer",
      "Updated": "Mis à jour",
      "Loading sessions…": "Chargement des sessions…",
      "Connect now": "Se connecter",
      "Authenticate once": "Authentifier une fois",
      "Forget": "Oublier",
      "Trusted machine detected. You can reconnect without retyping the host password.": "Machine de confiance détectée. Vous pouvez vous reconnecter sans ressaisir le mot de passe du host.",
      "Optional for this trusted machine": "Optionnel pour cette machine de confiance",
      "First approved connection will remember this host automatically for your account.": "La première connexion approuvée mémorisera automatiquement ce host pour votre compte.",
      "Unable to forget this trusted machine.": "Impossible d’oublier cette machine de confiance.",
      "Trusted machine forgotten. The host password will be required again.": "Machine de confiance oubliée. Le mot de passe du host sera de nouveau requis.",
      "Host target is required.": "La cible host est requise.",
      "This browser identity is not ready yet.": "L’identité de ce navigateur n’est pas encore prête.",
      "Enter the host password for the first approved connection.": "Saisissez le mot de passe du host pour la première connexion approuvée.",
      "Session {id} opened with trusted access.": "Session {id} ouverte avec accès de confiance.",
      "Session {id} opened. This host is now trusted for your account.": "Session {id} ouverte. Ce host est maintenant approuvé pour votre compte.",
      "trusted access": "accès approuvé",
      "first access uses host password": "le premier accès utilise le mot de passe du host",
      "first access needs the host password": "le premier accès nécessite le mot de passe du host",
      "No sessions created yet.": "Aucune session créée pour le moment.",
      "Profile": "Profil",
      "Preferences": "Préférences",
      "Identity": "Identité",
      "Control the avatar fallback, display name, and account email shown in the workspace.": "Contrôlez les initiales, le nom affiché et l’e-mail du compte visibles dans l’espace.",
      "Initials": "Initiales",
      "Save profile": "Enregistrer le profil",
      "Interface": "Interface",
      "Preferences are stored locally in this browser for a lighter and faster dashboard experience.": "Les préférences sont stockées localement dans ce navigateur pour un dashboard plus léger et rapide.",
      "Theme": "Thème",
      "System": "Système",
      "Light": "Clair",
      "Dark": "Sombre",
      "Default section": "Section par défaut",
      "Save preferences": "Enregistrer les préférences",
      "Host Bootstrap": "Bootstrap host",
      "Set the tenant domain and server route embedded in bootstrap packages. The universal host stays local until explicit provisioning credentials are provided.": "Définissez le domaine client et la route serveur embarqués dans les packages bootstrap. Le host universel reste local tant qu’aucun credential explicite n’est fourni.",
      "Set the tenant domain and server route used by the web claim API. The universal host stays local until this workspace explicitly links it through the loopback bridge.": "Définissez le domaine client et la route serveur utilisés par l’API web d’association. Le host universel reste local tant que cet espace ne le relie pas explicitement via le bridge loopback.",
      "Host Downloads": "Téléchargements host",
      "Downloads stay universal. If SERVER_URL is configured on the service, the claim API sends it to the local host. Otherwise this dashboard origin is used automatically.": "Les téléchargements restent universels. Si SERVER_URL est configurée sur le service, l’API d’association l’envoie au host local. Sinon, l’origine de ce dashboard est utilisée automatiquement.",
      "Install the host, open this account page on the same machine, then let the loopback bridge claim it once. No tenant-specific executable rebuild is generated.": "Installez le host, ouvrez cette page compte sur la même machine, puis laissez le bridge loopback l’associer une fois. Aucun exécutable spécifique à un client n’est regénéré.",
      "Bootstrap domain": "Domaine bootstrap",
      "Bootstrap server URL": "URL serveur bootstrap",
      "Save bootstrap": "Enregistrer le bootstrap",
      "Downloads below package the universal host together with unydesk-host.json. No tenant-specific executable rebuild is generated.": "Les téléchargements ci-dessous empaquettent le host universel avec unydesk-host.json. Aucun exécutable spécifique à un client n’est regénéré.",
      "Downloads below stay universal. Install the host, open this site on the same machine, then claim it once through the local API bridge. No tenant-specific executable rebuild is generated.": "Les téléchargements ci-dessous restent universels. Installez le host, ouvrez ce site sur la même machine, puis associez-le une fois via le bridge API local. Aucun exécutable spécifique à un client n’est regénéré.",
      "Windows 64-bit package": "Package Windows 64 bits",
      "Windows ARM64 package": "Package Windows ARM64",
      "Linux 64-bit package": "Package Linux 64 bits",
      "Linux ARM64 package": "Package Linux ARM64",
      "macOS Intel package": "Package macOS Intel",
      "macOS Apple Silicon package": "Package macOS Apple Silicon",
      "Windows 64-bit host": "Host Windows 64 bits",
      "Windows ARM64 host": "Host Windows ARM64",
      "Linux 64-bit host": "Host Linux 64 bits",
      "Linux ARM64 host": "Host Linux ARM64",
      "macOS Intel host": "Host macOS Intel",
      "macOS Apple Silicon host": "Host macOS Apple Silicon",
      "Security": "Sécurité",
      "Update the password used by this locally managed account.": "Modifiez le mot de passe utilisé par ce compte local.",
      "Current password": "Mot de passe actuel",
      "New password": "Nouveau mot de passe",
      "Update password": "Mettre à jour le mot de passe",
      "Session Control": "Contrôle de session",
      "Inspect and manage a remote-control session.": "Inspectez et gérez une session de contrôle distant.",
      "Live Session": "Session en direct",
      "Current control handshake and routing state.": "État actuel du handshake de contrôle et du routage.",
      "Session ID": "ID de session",
      "Created": "Créée",
      "Route": "Route",
      "Dispatch": "Dispatch",
      "Start viewer signaling": "Démarrer le signaling viewer",
      "Send ping": "Envoyer un ping",
      "Refresh": "Actualiser",
      "Close session": "Fermer la session",
      "Control Pad": "Surface de contrôle",
      "Focus the pad, then move, click, or type to send input events.": "Activez la surface, puis déplacez, cliquez ou tapez pour envoyer les événements.",
      "Remote control surface": "Surface de contrôle distant",
      "Signaling": "Signalisation",
      "Prepared for the upcoming host and viewer transport wiring.": "Préparé pour le câblage transport host/viewer à venir.",
      "Offer": "Offre",
      "Answer": "Réponse",
      "Viewer Transport": "Transport viewer",
      "ICE Candidates": "Candidats ICE",
      "Delivered At": "Livré à",
      "Deliveries": "Livraisons",
      "Host Acknowledged": "Host confirmé",
      "Diagnostics Logs": "Logs de diagnostic",
      "Keep this enabled only when debugging transport, latency, or signaling.": "Gardez cette option active uniquement pour déboguer transport, latence ou signaling.",
      "Enable only when debugging latency, codec fallback, signaling, clipboard, or file transfer.": "Activez uniquement pour déboguer latence, fallback codec, signaling, presse-papiers ou transfert de fichiers.",
      "Enable live logs": "Activer les logs live",
      "Transport Log": "Log transport",
      "Offer Preview": "Aperçu de l’offre",
      "Answer Preview": "Aperçu de la réponse",
      "Back to sessions": "Retour aux sessions",
      "Host Control": "Contrôle du host",
      "Inspect and drive a remote-control session in a dedicated workspace.": "Inspectez et pilotez une session de contrôle distant dans un espace dédié.",
      "Full screen": "Plein écran",
      "Host Screen": "Écran du host",
      "Click the screen, then move, click, or type to control the host.": "Cliquez sur l’écran, puis déplacez, cliquez ou tapez pour contrôler le host.",
      "No host screen yet.": "Aucun écran host pour le moment.",
      "Clipboard": "Presse-papiers",
      "Push text to the host, or read the current host clipboard into this workspace.": "Envoyez du texte au host ou lisez son presse-papiers dans cet espace.",
      "Shared text": "Texte partagé",
      "Send clipboard": "Envoyer le presse-papiers",
      "Read host clipboard": "Lire le presse-papiers du host",
      "File Transfer": "Transfert de fichiers",
      "Upload one file at a time directly to the host without leaving the control page.": "Envoyez un fichier à la fois directement au host sans quitter la page de contrôle.",
      "Choose file": "Choisir un fichier",
      "No file selected yet.": "Aucun fichier sélectionné.",
      "Send file": "Envoyer le fichier",
      "Cancel transfer": "Annuler le transfert",
      "Transport": "Transport",
      "WebRTC peer transport with brokered signaling and low-latency screen delivery.": "Transport pair-à-pair WebRTC avec signaling brokerisé et retour écran basse latence.",
      "Screen Path": "Chemin écran",
      "Control Path": "Chemin contrôle",
      "Live Features": "Fonctions live",
      "Protocol Notes": "Notes protocole",
      "Transport Notes": "Notes transport",
      "Back to landing": "Retour à l’accueil",
      "Open a direct viewer session from the landing page with an ephemeral client password.": "Ouvrez une session viewer directe depuis la landing avec un mot de passe client éphémère.",
      "Online": "En ligne",
      "Offline": "Hors ligne",
      "Paused": "En pause",
      "Access paused": "Accès en pause",
      "Control": "Contrôler",
      "Control host": "Contrôler le host",
      "Not available": "Indisponible",
      "No registered machines yet.": "Aucune machine enregistrée.",
      "Host target and browser client ID are required.": "La cible host et l’ID client du navigateur sont requis.",
      "Unable to create session.": "Impossible de créer la session.",
      "Session creation returned no session id.": "La création de session n’a retourné aucun ID.",
      "Unable to refresh session details right now.": "Impossible d’actualiser la session pour le moment.",
      "Unable to load session details right now.": "Impossible de charger la session pour le moment.",
      "Waiting for approval": "En attente d’autorisation",
      "The host must authorize this connection before live control opens.": "Le host doit autoriser cette connexion avant l’ouverture du contrôle à distance.",
      "Keep this window open while the host decides, or relaunch the request if the host popup was closed.": "Gardez cette fenêtre ouverte pendant que le host décide, ou relancez la demande si la popup du host a été fermée.",
      "Target host": "Host cible",
      "Pending session": "Session en attente",
      "Close waiting modal": "Fermer la fenêtre d’attente",
      "Waiting for host approval.": "En attente de l’autorisation du host.",
      "Waiting for host approval. Trusted access is ready once the host accepts.": "En attente de l’autorisation du host. L’accès approuvé sera prêt dès que le host accepte.",
      "Waiting for host approval. This host will be trusted for your account after the first accepted connection.": "En attente de l’autorisation du host. Ce host sera approuvé pour votre compte après la première connexion acceptée.",
      "Unable to re-open the approval request right now.": "Impossible de réafficher la demande d’autorisation pour le moment.",
      "Approval request sent to the host again.": "La demande d’autorisation a été renvoyée au host.",
      "Host approval was refused or the request expired.": "L’autorisation du host a été refusée ou la demande a expiré.",
      "Open live control": "Ouvrir le contrôle live",
      "Relaunch approval": "Relancer la demande",
      "Profile updated successfully.": "Profil mis à jour.",
      "Preferences saved in this browser.": "Préférences enregistrées dans ce navigateur.",
      "Unable to save bootstrap settings.": "Impossible d’enregistrer les paramètres bootstrap.",
      "Bootstrap settings saved. New host packages will include this domain and server route.": "Paramètres bootstrap enregistrés. Les nouveaux packages host incluront ce domaine et cette route serveur.",
      "Current and new password are required.": "Le mot de passe actuel et le nouveau sont requis."
    },
    de: {
      "Sign In": "Anmelden",
      "Go App · Alpine-friendly · API-first": "Go-App · Alpine-freundlich · API-first",
      "TRINITY Labs Remote Access": "TRINITY Labs Fernzugriff",
      "Free Remote Access": "Kostenloser Fernzugriff",
      "A lightweight remote-control broker built in Go for Alpine Linux, musl-friendly deployment, and clean future integration with UnyPort. You can expose this instance through a generated UnyDesk ID or keep a direct connection model with IP and port.": "Ein schlanker Fernsteuerungs-Broker in Go für Alpine Linux, musl-freundliches Deployment und eine saubere spätere Integration mit UnyPort. Diese Instanz kann über eine generierte UnyDesk-ID oder direkt per IP und Port erreichbar sein.",
      "Client IP": "Client-IP",
      "Direct Address": "Direkte Adresse",
      "Runtime": "Laufzeit",
      "Version": "Version",
      "Server": "Server",
      "Loading…": "Wird geladen...",
      "Detecting...": "Erkennung...",
      "Detecting…": "Erkennung...",
      "UnyDesk ID": "UnyDesk-ID",
      "Host Password": "Host-Passwort",
      "Ephemeral Client Password": "Temporäres Client-Passwort",
      "ID copied!": "ID kopiert!",
      "Password copied!": "Passwort kopiert!",
      "Open API Info": "API-Info öffnen",
      "Health Check": "Systemprüfung",
      "Detecting system": "System wird erkannt",
      "Preparing suggestion…": "Empfehlung wird vorbereitet...",
      "Preparing suggestion...": "Empfehlung wird vorbereitet...",
      "Waiting for browser detection.": "Browser-Erkennung läuft.",
      "Detected in your browser. If needed, open the full download list.": "In Ihrem Browser erkannt. Öffnen Sie bei Bedarf die vollständige Downloadliste.",
      "All host downloads": "Alle Host-Downloads",
      "Windows builds include metadata and SHA256 checksums. SmartScreen warnings may still appear until code signing is enabled.": "Windows-Builds enthalten Metadaten und SHA256-Prüfsummen. SmartScreen-Warnungen können erscheinen, bis Code-Signing aktiviert ist.",
      "Windows builds include metadata and": "Windows-Builds enthalten Metadaten und",
      "checksums. SmartScreen warnings may still appear until code signing is enabled.": "Prüfsummen. SmartScreen-Warnungen können erscheinen, bis Code-Signing aktiviert ist.",
      "Quick host test:": "Schneller Host-Test:",
      "Connect to Host": "Mit Host verbinden",
      "Enter the remote UnyDesk ID and the password currently shown by the host binary on its local browser page.": "Geben Sie die entfernte UnyDesk-ID und das Passwort ein, das derzeit vom Host-Binary auf seiner lokalen Browserseite angezeigt wird.",
      "Standalone Client": "Standalone-Client",
      "Open a direct viewer from this landing page with your local client ID and an ephemeral password.": "Öffnen Sie direkt von dieser Seite einen Viewer mit lokaler Client-ID und temporärem Passwort.",
      "Client ID": "Client-ID",
      "Target Host ID": "Ziel-Host-ID",
      "Current password shown on the host": "Aktuell auf dem Host angezeigtes Passwort",
      "Connect to host": "Mit Host verbinden",
      "Open standalone client": "Standalone-Client öffnen",
      "Opening...": "Wird geöffnet...",
      "No account required. Open a standalone client in a new tab.": "Kein Konto erforderlich. Öffnen Sie einen Standalone-Client in einem neuen Tab.",
      "Self-hosted remote access by TRINITY Edge Networks.": "Self-hosted Fernzugriff von TRINITY Edge Networks.",
      "Next step: secure connect flow and real remote-control transport.": "Nächster Schritt: sicherer Verbindungsfluss und echter Fernsteuerungs-Transport.",
      "Host Downloads": "Host-Downloads",
      "Manual platform selection stays available even when automatic detection is uncertain.": "Die manuelle Plattformauswahl bleibt verfügbar, wenn die automatische Erkennung unsicher ist.",
      "Close": "Schließen",
      "Download": "Herunterladen",
      "ZIP": "ZIP",
      "Checksums": "Prüfsummen",
      "Open SHA256SUMS": "SHA256SUMS öffnen",
      "Load in modal": "Im Fenster laden",
      "Checksums will appear here on demand.": "Prüfsummen erscheinen hier bei Bedarf.",
      "Loading checksums...": "Prüfsummen werden geladen...",
      "No checksums available.": "Keine Prüfsummen verfügbar.",
      "Unable to load checksums right now.": "Prüfsummen können derzeit nicht geladen werden.",
      "Accounts are managed locally through users.json. Public sign-up is disabled.": "Konten werden lokal über users.json verwaltet. Öffentliche Registrierung ist deaktiviert.",
      "Email": "E-Mail",
      "Password": "Passwort",
      "Service Status": "Dienststatus",
      "Inspect API and health endpoint responses without leaving the page.": "Prüfen Sie API- und Health-Endpunkt-Antworten, ohne die Seite zu verlassen.",
      "Ready.": "Bereit.",
      "Choose an endpoint to inspect.": "Wählen Sie einen Endpunkt zur Prüfung.",
      "Available": "Verfügbar",
      "Unavailable": "Nicht verfügbar",
      "Loading...": "Wird geladen...",
      "API Info": "API-Info",
      "Live response from the service health endpoint.": "Live-Antwort des Health-Endpunkts.",
      "Live response from the API information endpoint.": "Live-Antwort des API-Informationsendpunkts.",
      "No response body.": "Kein Antwortinhalt.",
      "The request failed before a response was received.": "Die Anfrage ist fehlgeschlagen, bevor eine Antwort empfangen wurde.",
      "Unable to open a standalone session right now.": "Standalone-Sitzung kann derzeit nicht geöffnet werden.",
      "Enter a target host ID or hostname first.": "Geben Sie zuerst eine Ziel-Host-ID oder einen Hostnamen ein.",
      "Client identity is still loading.": "Client-Identität wird noch geladen.",
      "Enter the host password first.": "Geben Sie zuerst das Host-Passwort ein.",
      "Authenticating host access...": "Host-Zugriff wird authentifiziert...",
      "Security token unavailable. Refresh the page and try again.": "Sicherheits-Token nicht verfügbar. Aktualisieren Sie die Seite und versuchen Sie es erneut.",
      "Client not installed on this device": "Client ist auf diesem Gerät nicht installiert",
      "Unavailable": "Nicht verfügbar",
      "Detected locally from the installed host client.": "Lokal über den installierten Host-Client erkannt.",
      "Local host detected. Claim it once to bind this machine to your workspace.": "Lokaler Host erkannt. Einmal zuordnen, um diese Maschine an Ihren Workspace zu binden.",
      "Sign in to claim this local host and keep it attached to your workspace.": "Melden Sie sich an, um diesen lokalen Host zuzuordnen und dauerhaft an Ihren Workspace zu binden.",
      "Claim local host": "Lokalen Host zuordnen",
      "Sign in to claim": "Zum Zuordnen anmelden",
      "Linking...": "Wird verknüpft...",
      "Linking the local host to your workspace...": "Der lokale Host wird mit Ihrem Workspace verknüpft...",
      "Local host linked to your workspace.": "Lokaler Host mit Ihrem Workspace verknüpft.",
      "Unable to claim the local host right now.": "Der lokale Host kann gerade nicht zugeordnet werden.",
      "The public landing page only shows the host password when the local host client is installed on this device.": "Die öffentliche Landingpage zeigt das Host-Passwort nur an, wenn der lokale Host-Client auf diesem Gerät installiert ist.",
      "Host password rotated locally.": "Host-Passwort lokal erneuert.",
      "Unable to rotate the local host password right now.": "Das lokale Host-Passwort kann derzeit nicht erneuert werden.",
      "Preparing standalone session...": "Standalone-Sitzung wird vorbereitet...",
      "Ephemeral client password regenerated.": "Temporäres Client-Passwort wurde neu erzeugt.",
      "Recommended host: Windows 64-bits": "Empfohlener Host: Windows 64-Bit",
      "Recommended host: Windows ARM64": "Empfohlener Host: Windows ARM64",
      "Recommended host: macOS Intel": "Empfohlener Host: macOS Intel",
      "Recommended host: macOS Apple Silicon": "Empfohlener Host: macOS Apple Silicon",
      "Recommended host: Linux 64-bits": "Empfohlener Host: Linux 64-Bit",
      "Download Windows 64-bits": "Windows 64-Bit herunterladen",
      "Download Windows ARM64": "Windows ARM64 herunterladen",
      "Download macOS Intel": "macOS Intel herunterladen",
      "Download macOS Apple Silicon": "macOS Apple Silicon herunterladen",
      "Download Linux 64-bits": "Linux 64-Bit herunterladen",
      "Android detected": "Android erkannt",
      "Android host coming soon": "Android-Host bald verfügbar",
      "Detected: Windows 64-bits.": "Erkannt: Windows 64-Bit.",
      "Detected: Windows on ARM.": "Erkannt: Windows auf ARM.",
      "Detected: macOS Intel.": "Erkannt: macOS Intel.",
      "Detected: macOS Apple Silicon.": "Erkannt: macOS Apple Silicon.",
      "Detection was unclear. This is the fallback choice.": "Die Erkennung war unsicher. Dies ist die Standardauswahl.",
      "Detected: Linux 64-bits. Distribution not exposed by this browser.": "Erkannt: Linux 64-Bit. Die Distribution wird von diesem Browser nicht offengelegt.",
      "Detected: Linux ARM64. Distribution not exposed by this browser.": "Erkannt: Linux ARM64. Die Distribution wird von diesem Browser nicht offengelegt.",
      "Detected: Android. Native host APK is not published yet; use the standalone web client from this browser for now.": "Erkannt: Android. Das native Host-APK ist noch nicht veröffentlicht; verwenden Sie vorerst den Standalone-Webclient in diesem Browser.",
      "Host for Linux 64-bits": "Host für Linux 64-Bit",
      "Host for Linux ARM64": "Host für Linux ARM64",
      "Host for Android": "Host für Android",
      "Host for Windows 64-bits": "Host für Windows 64-Bit",
      "Host for Windows ARM64": "Host für Windows ARM64",
      "Host for macOS Intel": "Host für macOS Intel",
      "Host for macOS Apple Silicon": "Host für macOS Apple Silicon",
      "Static binary for major x86_64 Linux distributions.": "Statisches Binary für wichtige x86_64-Linux-Distributionen.",
      "Same host flow for ARM targets and lightweight edge nodes.": "Gleicher Host-Flow für ARM-Ziele und schlanke Edge-Knoten.",
      "Android endpoint detected. Native APK is not published yet; use the standalone web client for now.": "Android-Endpunkt erkannt. Die native APK ist noch nicht veröffentlicht; nutzen Sie vorerst den Standalone-Webclient.",
      "Standard 64-bit Windows host binary for desktop and server editions.": "Standardmäßiges 64-Bit-Windows-Host-Binary für Desktop- und Server-Editionen.",
      "Windows on ARM build for newer ARM laptops and tablets.": "Windows-on-ARM-Build für neuere ARM-Laptops und Tablets.",
      "Darwin build for Intel-based Mac systems.": "Darwin-Build für Intel-basierte Mac-Systeme.",
      "Darwin build for Apple Silicon systems.": "Darwin-Build für Apple-Silicon-Systeme.",
      "Coming soon": "Demnächst",

      "Workspace": "Arbeitsbereich",
      "Overview": "Übersicht",
      "Connect": "Verbinden",
      "Machines": "Maschinen",
      "Sessions": "Sitzungen",
      "Account": "Konto",
      "User Settings": "Benutzereinstellungen",
      "Sign out": "Abmelden",
      "Back to home": "Zur Startseite",
      "Remote Access Workspace": "Fernzugriff-Arbeitsbereich",
      "Connect with a one-time UnyDesk ID and password, then keep trusted machines registered for future access.": "Verbinden Sie sich mit einer einmaligen UnyDesk-ID und einem Passwort und registrieren Sie vertrauenswürdige Maschinen für spätere Zugriffe.",
      "Loading session…": "Sitzung wird geladen…",
      "Display name": "Anzeigename",
      "Authenticated user": "Authentifizierter Benutzer",
      "Browser Client ID": "Browser-Client-ID",
      "Ready hosts": "Bereite Hosts",
      "Registered Access": "Registrierter Zugriff",
      "Machines saved for repeat access stay visible even when offline.": "Gespeicherte Maschinen bleiben auch offline sichtbar.",
      "Registered machines": "Registrierte Maschinen",
      "Not ready": "Nicht bereit",
      "Connection Activity": "Verbindungsaktivität",
      "Reusable session records for active and recent remote access.": "Wiederverwendbare Sitzungsdatensätze für aktive und aktuelle Fernzugriffe.",
      "Live sessions": "Live-Sitzungen",
      "Session records": "Sitzungsdatensätze",
      "Active, paused, and offline hosts from your saved fleet.": "Aktive, pausierte und offline Hosts aus Ihrer gespeicherten Flotte.",
      "Loading registered machines…": "Registrierte Maschinen werden geladen…",
      "Connect to a Host": "Mit Host verbinden",
      "Enter the host ID and the password currently displayed by the host binary. After the first approved access, trusted machines can reconnect without retyping the password.": "Geben Sie die Host-ID und das aktuell vom Host-Binary angezeigte Passwort ein. Nach dem ersten freigegebenen Zugriff können vertrauenswürdige Maschinen sich ohne erneute Passworteingabe verbinden.",
      "Host ID or registered machine": "Host-ID oder registrierte Maschine",
      "Connect or reuse access": "Verbinden oder Zugriff wiederverwenden",
      "The first authenticated connection remembers this host for your account automatically.": "Die erste authentifizierte Verbindung merkt sich diesen Host automatisch für Ihr Konto.",
      "Registered Machines": "Registrierte Maschinen",
      "Review saved machines, access state, platform, and last heartbeat before opening a session.": "Prüfen Sie gespeicherte Maschinen, Zugriffsstatus, Plattform und letzten Heartbeat, bevor Sie eine Sitzung öffnen.",
      "Machine": "Maschine",
      "Platform": "Plattform",
      "Status": "Status",
      "Action": "Aktion",
      "Loading hosts…": "Hosts werden geladen…",
      "Session Manager": "Sitzungsmanager",
      "Monitor pending, active, and historical remote connections from one place.": "Überwachen Sie ausstehende, aktive und historische Fernverbindungen an einem Ort.",
      "Session": "Sitzung",
      "Target": "Ziel",
      "Viewer": "Viewer",
      "Updated": "Aktualisiert",
      "Loading sessions…": "Sitzungen werden geladen…",
      "Connect now": "Jetzt verbinden",
      "Authenticate once": "Einmal authentifizieren",
      "Forget": "Vergessen",
      "Trusted machine detected. You can reconnect without retyping the host password.": "Vertrauenswürdige Maschine erkannt. Sie können sich ohne erneute Eingabe des Host-Passworts verbinden.",
      "Optional for this trusted machine": "Optional für diese vertrauenswürdige Maschine",
      "First approved connection will remember this host automatically for your account.": "Die erste freigegebene Verbindung merkt sich diesen Host automatisch für Ihr Konto.",
      "Unable to forget this trusted machine.": "Diese vertrauenswürdige Maschine kann derzeit nicht vergessen werden.",
      "Trusted machine forgotten. The host password will be required again.": "Vertrauenswürdige Maschine vergessen. Das Host-Passwort wird wieder benötigt.",
      "Host target is required.": "Ein Host-Ziel ist erforderlich.",
      "This browser identity is not ready yet.": "Die Browser-Identität ist noch nicht bereit.",
      "Enter the host password for the first approved connection.": "Geben Sie für die erste freigegebene Verbindung das Host-Passwort ein.",
      "Session {id} opened with trusted access.": "Sitzung {id} mit Vertrauenszugriff geöffnet.",
      "Session {id} opened. This host is now trusted for your account.": "Sitzung {id} geöffnet. Dieser Host ist jetzt für Ihr Konto vertrauenswürdig.",
      "trusted access": "vertrauenswürdiger Zugriff",
      "first access uses host password": "beim ersten Zugriff wird das Host-Passwort verwendet",
      "first access needs the host password": "beim ersten Zugriff wird das Host-Passwort benötigt",
      "No sessions created yet.": "Es wurden noch keine Sitzungen erstellt.",
      "Profile": "Profil",
      "Preferences": "Einstellungen",
      "Identity": "Identität",
      "Control the avatar fallback, display name, and account email shown in the workspace.": "Steuern Sie Initialen, Anzeigename und Konto-E-Mail im Arbeitsbereich.",
      "Initials": "Initialen",
      "Save profile": "Profil speichern",
      "Interface": "Oberfläche",
      "Preferences are stored locally in this browser for a lighter and faster dashboard experience.": "Einstellungen werden lokal in diesem Browser gespeichert, für ein leichteres und schnelleres Dashboard.",
      "Theme": "Design",
      "System": "System",
      "Light": "Hell",
      "Dark": "Dunkel",
      "Default section": "Standardbereich",
      "Save preferences": "Einstellungen speichern",
      "Host Bootstrap": "Host-Bootstrap",
      "Set the tenant domain and server route embedded in bootstrap packages. The universal host stays local until explicit provisioning credentials are provided.": "Legen Sie die Mandanten-Domain und die Server-Route fest, die in Bootstrap-Pakete eingebettet werden. Der universelle Host bleibt lokal, bis explizite Bereitstellungsdaten angegeben werden.",
      "Set the tenant domain and server route used by the web claim API. The universal host stays local until this workspace explicitly links it through the loopback bridge.": "Legen Sie Mandanten-Domain und Serverroute fest, die von der Web-Claim-API verwendet werden. Der universelle Host bleibt lokal, bis dieser Workspace ihn explizit über die Loopback-Bridge verknüpft.",
      "Host Downloads": "Host-Downloads",
      "Downloads stay universal. If SERVER_URL is configured on the service, the claim API sends it to the local host. Otherwise this dashboard origin is used automatically.": "Die Downloads bleiben universell. Wenn SERVER_URL im Dienst gesetzt ist, sendet die Claim-API sie an den lokalen Host. Andernfalls wird der Ursprung dieses Dashboards automatisch verwendet.",
      "Install the host, open this account page on the same machine, then let the loopback bridge claim it once. No tenant-specific executable rebuild is generated.": "Installieren Sie den Host, öffnen Sie diese Kontoseite auf derselben Maschine und lassen Sie ihn dann einmal über die Loopback-Bridge verknüpfen. Es wird kein mandantenspezifisches Binary neu erzeugt.",
      "Bootstrap domain": "Bootstrap-Domain",
      "Bootstrap server URL": "Bootstrap-Server-URL",
      "Save bootstrap": "Bootstrap speichern",
      "Downloads below package the universal host together with unydesk-host.json. No tenant-specific executable rebuild is generated.": "Die folgenden Downloads paketieren den universellen Host zusammen mit unydesk-host.json. Es wird kein kundenspezifisches Binary neu erzeugt.",
      "Downloads below stay universal. Install the host, open this site on the same machine, then claim it once through the local API bridge. No tenant-specific executable rebuild is generated.": "Die folgenden Downloads bleiben universell. Installieren Sie den Host, öffnen Sie diese Website auf derselben Maschine und ordnen Sie ihn dann einmal über die lokale API-Bridge zu. Es wird kein mandantenspezifisches Exe neu erstellt.",
      "Windows 64-bit package": "Windows-64-Bit-Paket",
      "Windows ARM64 package": "Windows-ARM64-Paket",
      "Linux 64-bit package": "Linux-64-Bit-Paket",
      "Linux ARM64 package": "Linux-ARM64-Paket",
      "macOS Intel package": "macOS-Intel-Paket",
      "macOS Apple Silicon package": "macOS-Apple-Silicon-Paket",
      "Windows 64-bit host": "Windows-64-Bit-Host",
      "Windows ARM64 host": "Windows-ARM64-Host",
      "Linux 64-bit host": "Linux-64-Bit-Host",
      "Linux ARM64 host": "Linux-ARM64-Host",
      "macOS Intel host": "macOS-Intel-Host",
      "macOS Apple Silicon host": "macOS-Apple-Silicon-Host",
      "Security": "Sicherheit",
      "Update the password used by this locally managed account.": "Aktualisieren Sie das Passwort dieses lokal verwalteten Kontos.",
      "Current password": "Aktuelles Passwort",
      "New password": "Neues Passwort",
      "Update password": "Passwort aktualisieren",
      "Session Control": "Sitzungssteuerung",
      "Inspect and manage a remote-control session.": "Prüfen und verwalten Sie eine Fernsteuerungssitzung.",
      "Live Session": "Live-Sitzung",
      "Current control handshake and routing state.": "Aktueller Steuerungs-Handshake und Routingstatus.",
      "Session ID": "Sitzungs-ID",
      "Created": "Erstellt",
      "Route": "Route",
      "Dispatch": "Dispatch",
      "Start viewer signaling": "Viewer-Signalisierung starten",
      "Send ping": "Ping senden",
      "Refresh": "Aktualisieren",
      "Close session": "Sitzung schließen",
      "Control Pad": "Steuerfläche",
      "Focus the pad, then move, click, or type to send input events.": "Fokussieren Sie die Fläche, dann bewegen, klicken oder tippen Sie, um Eingaben zu senden.",
      "Remote control surface": "Fernsteuerungsfläche",
      "Signaling": "Signalisierung",
      "Prepared for the upcoming host and viewer transport wiring.": "Vorbereitet für die kommende Host- und Viewer-Transportverkabelung.",
      "Offer": "Angebot",
      "Answer": "Antwort",
      "Viewer Transport": "Viewer-Transport",
      "ICE Candidates": "ICE-Kandidaten",
      "Delivered At": "Geliefert um",
      "Deliveries": "Lieferungen",
      "Host Acknowledged": "Host bestätigt",
      "Diagnostics Logs": "Diagnose-Logs",
      "Keep this enabled only when debugging transport, latency, or signaling.": "Nur aktiviert lassen, wenn Transport, Latenz oder Signalisierung debuggt werden.",
      "Enable only when debugging latency, codec fallback, signaling, clipboard, or file transfer.": "Nur aktivieren, wenn Latenz, Codec-Fallback, Signalisierung, Zwischenablage oder Dateitransfer debuggt werden.",
      "Enable live logs": "Live-Logs aktivieren",
      "Transport Log": "Transport-Log",
      "Offer Preview": "Angebotsvorschau",
      "Answer Preview": "Antwortvorschau",
      "Back to sessions": "Zurück zu Sitzungen",
      "Host Control": "Host-Steuerung",
      "Inspect and drive a remote-control session in a dedicated workspace.": "Prüfen und steuern Sie eine Fernzugriffssitzung in einem eigenen Arbeitsbereich.",
      "Full screen": "Vollbild",
      "Host Screen": "Host-Bildschirm",
      "Click the screen, then move, click, or type to control the host.": "Klicken Sie auf den Bildschirm, dann bewegen, klicken oder tippen Sie, um den Host zu steuern.",
      "No host screen yet.": "Noch kein Host-Bildschirm.",
      "Clipboard": "Zwischenablage",
      "Push text to the host, or read the current host clipboard into this workspace.": "Senden Sie Text an den Host oder lesen Sie die aktuelle Host-Zwischenablage in diesen Arbeitsbereich.",
      "Shared text": "Geteilter Text",
      "Send clipboard": "Zwischenablage senden",
      "Read host clipboard": "Host-Zwischenablage lesen",
      "File Transfer": "Dateitransfer",
      "Upload one file at a time directly to the host without leaving the control page.": "Laden Sie jeweils eine Datei direkt zum Host hoch, ohne die Steuerungsseite zu verlassen.",
      "Choose file": "Datei auswählen",
      "No file selected yet.": "Noch keine Datei ausgewählt.",
      "Send file": "Datei senden",
      "Cancel transfer": "Transfer abbrechen",
      "Transport": "Transport",
      "WebRTC peer transport with brokered signaling and low-latency screen delivery.": "WebRTC-Peer-Transport mit brokerbasierter Signalisierung und latenzarmer Bildschirmübertragung.",
      "Screen Path": "Bildschirm-Pfad",
      "Control Path": "Steuerpfad",
      "Live Features": "Live-Funktionen",
      "Protocol Notes": "Protokollnotizen",
      "Transport Notes": "Transportnotizen",
      "Back to landing": "Zurück zur Startseite",
      "Open a direct viewer session from the landing page with an ephemeral client password.": "Öffnen Sie eine direkte Viewer-Sitzung von der Landingpage mit temporärem Client-Passwort.",
      "Online": "Online",
      "Offline": "Offline",
      "Paused": "Pausiert",
      "Access paused": "Zugriff pausiert",
      "Control": "Steuern",
      "Control host": "Host steuern",
      "Not available": "Nicht verfügbar",
      "No registered machines yet.": "Noch keine registrierten Maschinen.",
      "Host target and browser client ID are required.": "Host-Ziel und Browser-Client-ID sind erforderlich.",
      "Unable to create session.": "Sitzung kann nicht erstellt werden.",
      "Session creation returned no session id.": "Sitzungserstellung lieferte keine ID zurück.",
      "Unable to refresh session details right now.": "Sitzungsdetails können derzeit nicht aktualisiert werden.",
      "Unable to load session details right now.": "Sitzungsdetails können derzeit nicht geladen werden.",
      "Waiting for approval": "Warten auf Freigabe",
      "The host must authorize this connection before live control opens.": "Der Host muss diese Verbindung autorisieren, bevor die Live-Steuerung geöffnet wird.",
      "Keep this window open while the host decides, or relaunch the request if the host popup was closed.": "Lassen Sie dieses Fenster offen, während der Host entscheidet, oder senden Sie die Anfrage erneut, falls das Host-Popup geschlossen wurde.",
      "Target host": "Ziel-Host",
      "Pending session": "Ausstehende Sitzung",
      "Close waiting modal": "Wartefenster schließen",
      "Waiting for host approval.": "Warten auf die Bestätigung des Hosts.",
      "Waiting for host approval. Trusted access is ready once the host accepts.": "Warten auf die Bestätigung des Hosts. Der vertrauenswürdige Zugriff ist bereit, sobald der Host akzeptiert.",
      "Waiting for host approval. This host will be trusted for your account after the first accepted connection.": "Warten auf die Bestätigung des Hosts. Dieser Host wird nach der ersten akzeptierten Verbindung für Ihr Konto als vertrauenswürdig gespeichert.",
      "Unable to re-open the approval request right now.": "Die Bestätigungsanfrage kann derzeit nicht erneut angezeigt werden.",
      "Approval request sent to the host again.": "Die Bestätigungsanfrage wurde erneut an den Host gesendet.",
      "Host approval was refused or the request expired.": "Die Host-Bestätigung wurde abgelehnt oder die Anfrage ist abgelaufen.",
      "Open live control": "Live-Steuerung öffnen",
      "Relaunch approval": "Bestätigung erneut anzeigen",
      "Profile updated successfully.": "Profil erfolgreich aktualisiert.",
      "Preferences saved in this browser.": "Einstellungen in diesem Browser gespeichert.",
      "Unable to save bootstrap settings.": "Bootstrap-Einstellungen können nicht gespeichert werden.",
      "Bootstrap settings saved. New host packages will include this domain and server route.": "Bootstrap-Einstellungen gespeichert. Neue Host-Pakete enthalten diese Domain und Server-Route.",
      "Current and new password are required.": "Aktuelles und neues Passwort sind erforderlich."
    }
  };

  const textOriginals = new WeakMap();
  let currentLocale = normalizeLocale(readStoredLocale() || detectBrowserLocale());
  let applying = false;

  function readStoredLocale() {
    try {
      return window.localStorage.getItem(STORAGE_KEY) || "";
    } catch (_error) {
      return "";
    }
  }

  function storeLocale(locale) {
    try {
      window.localStorage.setItem(STORAGE_KEY, locale);
    } catch (_error) {
    }
  }

  function detectBrowserLocale() {
    const source = `${navigator.language || ""}`.toLowerCase();
    if (source.startsWith("fr")) return "fr";
    if (source.startsWith("de")) return "de";
    return "en";
  }

  function normalizeLocale(locale) {
    const normalized = String(locale || "").trim().toLowerCase().slice(0, 2);
    return SUPPORTED_LOCALES.includes(normalized) ? normalized : "en";
  }

  function normalizeKey(value) {
    return String(value || "").replace(/\s+/g, " ").trim();
  }

  function interpolate(value, params = {}) {
    return String(value || "").replace(/\{([a-zA-Z0-9_]+)\}/g, (match, key) => {
      return Object.prototype.hasOwnProperty.call(params, key) ? String(params[key]) : match;
    });
  }

  function dynamicTranslation(key) {
    if (currentLocale === "en") return "";

    const loadingEndpoint = key.match(/^Loading (.+)\.\.\.$/);
    if (loadingEndpoint) {
      return currentLocale === "fr"
        ? `Chargement de ${loadingEndpoint[1]}...`
        : `${loadingEndpoint[1]} wird geladen...`;
    }

    const unableEndpoint = key.match(/^Unable to reach (.+)$/);
    if (unableEndpoint) {
      return currentLocale === "fr"
        ? `Impossible de joindre ${unableEndpoint[1]}`
        : `${unableEndpoint[1]} ist nicht erreichbar`;
    }

    const linuxRecommendation = key.match(/^Recommended host: Linux (.+)$/);
    if (linuxRecommendation) {
      return currentLocale === "fr"
        ? `Host recommandé : Linux ${linuxRecommendation[1].replace("64-bits", "64 bits")}`
        : `Empfohlener Host: Linux ${linuxRecommendation[1].replace("64-bits", "64-Bit")}`;
    }

    const linuxDownload = key.match(/^Download Linux (.+)$/);
    if (linuxDownload) {
      return currentLocale === "fr"
        ? `Télécharger Linux ${linuxDownload[1].replace("64-bits", "64 bits")}`
        : `Linux ${linuxDownload[1].replace("64-bits", "64-Bit")} herunterladen`;
    }

    const linuxDetected = key.match(/^Detected: (.+) Linux (.+)\.$/);
    if (linuxDetected) {
      return currentLocale === "fr"
        ? `Détecté : ${linuxDetected[1]} Linux ${linuxDetected[2].replace("64-bits", "64 bits")}.`
        : `Erkannt: ${linuxDetected[1]} Linux ${linuxDetected[2].replace("64-bits", "64-Bit")}.`;
    }

    const linuxGeneric = key.match(/^Detected: Linux (.+)\. Distribution not exposed by this browser\.$/);
    if (linuxGeneric) {
      return currentLocale === "fr"
        ? `Détecté : Linux ${linuxGeneric[1].replace("64-bits", "64 bits")}. Distribution non exposée par ce navigateur.`
        : `Erkannt: Linux ${linuxGeneric[1].replace("64-bits", "64-Bit")}. Die Distribution wird von diesem Browser nicht offengelegt.`;
    }

    const androidRecommendation = key.match(/^Android detected(.*)$/);
    if (androidRecommendation) {
      return currentLocale === "fr"
        ? `Android détecté${androidRecommendation[1]}`
        : `Android erkannt${androidRecommendation[1]}`;
    }

    const androidDetected = key.match(/^Detected: Android(.*)\. Native host APK is not published yet; use the standalone web client from this browser for now\.$/);
    if (androidDetected) {
      return currentLocale === "fr"
        ? `Détecté : Android${androidDetected[1]}. L’APK host natif n’est pas encore publié ; utilisez le client web autonome depuis ce navigateur pour le moment.`
        : `Erkannt: Android${androidDetected[1]}. Das native Host-APK ist noch nicht veröffentlicht; verwenden Sie vorerst den Standalone-Webclient in diesem Browser.`;
    }

    return "";
  }

  function t(key, fallback = key, params = {}) {
    const normalized = normalizeKey(key);
    if (!normalized) return "";
    const translated = dictionaries[currentLocale] && dictionaries[currentLocale][normalized];
    return interpolate(translated || dynamicTranslation(normalized) || fallback || normalized, params);
  }

  function shouldSkipTextNode(node) {
    const parent = node && node.parentElement;
    if (!parent) return true;
    if (parent.closest("script, style, code, pre, textarea, [data-i18n-skip]")) return true;
    return normalizeKey(node.nodeValue) === "";
  }

  function translateTextNode(node, options = {}) {
    if (shouldSkipTextNode(node)) return;
    if (options.refreshOriginal) {
      textOriginals.set(node, normalizeKey(node.nodeValue));
    }
    const original = textOriginals.get(node) || normalizeKey(node.nodeValue);
    if (!textOriginals.has(node)) textOriginals.set(node, original);
    const translated = t(original, original);
    const value = String(node.nodeValue || "");
    const leading = value.match(/^\s*/)[0];
    const trailing = value.match(/\s*$/)[0];
    node.nodeValue = `${leading}${translated}${trailing}`;
  }

  function translateAttributes(root) {
    const attrs = ["placeholder", "title", "aria-label"];
    const nodes = root.querySelectorAll ? root.querySelectorAll("input, textarea, button, a, [title], [aria-label]") : [];
    nodes.forEach((node) => {
      attrs.forEach((attr) => {
        if (!node.hasAttribute(attr)) return;
        const dataName = `i18nOriginal${attr.replace(/-([a-z])/g, (_, char) => char.toUpperCase())}`;
        if (!node.dataset[dataName]) node.dataset[dataName] = node.getAttribute(attr) || "";
        const original = node.dataset[dataName];
        node.setAttribute(attr, t(original, original));
      });
    });
  }

  function translateTree(root) {
    if (!root) return;
    applying = true;
    try {
      if (root.nodeType === Node.TEXT_NODE) {
        translateTextNode(root);
      } else {
        const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
        let node = walker.nextNode();
        while (node) {
          translateTextNode(node);
          node = walker.nextNode();
        }
        translateAttributes(root);
      }
      updateSwitcherState(root);
      document.documentElement.lang = currentLocale;
    } finally {
      applying = false;
    }
  }

  function updateSwitcherState(root = document) {
    const buttons = root.querySelectorAll ? root.querySelectorAll("[data-locale]") : [];
    buttons.forEach((button) => {
      const locale = normalizeLocale(button.dataset.locale);
      button.classList.toggle("is-active", locale === currentLocale);
      button.setAttribute("aria-pressed", locale === currentLocale ? "true" : "false");
      button.setAttribute("title", LOCALE_LABELS[locale] || locale.toUpperCase());
    });
  }

  function bindSwitchers(root = document) {
    const buttons = root.querySelectorAll ? root.querySelectorAll("[data-locale]") : [];
    buttons.forEach((button) => {
      if (button.dataset.localeBound === "1") return;
      button.dataset.localeBound = "1";
      button.addEventListener("click", () => {
        setLocale(button.dataset.locale);
      });
    });
  }

  function setLocale(locale) {
    currentLocale = normalizeLocale(locale);
    storeLocale(currentLocale);
    bindSwitchers(document);
    translateTree(document.body || document.documentElement);
    window.dispatchEvent(new CustomEvent("unydesk:localechange", { detail: { locale: currentLocale } }));
  }

  window.unydeskI18n = {
    t,
    setLocale,
    getLocale: () => currentLocale,
    apply: translateTree,
    supportedLocales: () => SUPPORTED_LOCALES.slice(),
  };

  document.addEventListener("DOMContentLoaded", () => {
    bindSwitchers(document);
    translateTree(document.body || document.documentElement);
  });
})();
