// Service Worker for Ourocodus PWA
// Precaches hashed assets for offline support

const CACHE_NAME = 'ourocodus-v1';
const MANIFEST_URL = '/asset-manifest.json';

// Install event - precache all hashed assets
self.addEventListener('install', (event) => {
  event.waitUntil(
    fetch(MANIFEST_URL)
      .then(response => response.json())
      .then(manifest => {
        // Get all hashed filenames from manifest
        const assetsToCache = [
          '/',
          '/index.html',
          '/manifest.webmanifest',
          ...Object.values(manifest).map(hashedName => `/${hashedName}`)
        ];

        console.log('[SW] Precaching assets:', assetsToCache);

        return caches.open(CACHE_NAME).then(cache => {
          return cache.addAll(assetsToCache);
        });
      })
      .catch(err => {
        console.error('[SW] Failed to precache assets:', err);
        throw err; // Rethrow to fail install so previous worker stays active
      })
  );

  // Activate immediately
  self.skipWaiting();
});

// Activate event - clean up old caches
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then(cacheNames => {
      return Promise.all(
        cacheNames
          .filter(name => name !== CACHE_NAME)
          .map(name => {
            console.log('[SW] Deleting old cache:', name);
            return caches.delete(name);
          })
      );
    })
  );

  // Take control of all pages immediately
  return self.clients.claim();
});

// Fetch event - cache-first strategy for hashed assets, network-first for others
self.addEventListener('fetch', (event) => {
  const url = new URL(event.request.url);

  // Skip non-GET requests and different origins
  if (event.request.method !== 'GET' || url.origin !== self.location.origin) {
    return;
  }

  // Skip WebSocket connections
  if (url.protocol === 'ws:' || url.protocol === 'wss:') {
    return;
  }

  // Cache-first strategy for hashed assets (they never change)
  if (url.pathname.match(/\.[a-f0-9]{8}\.(js|css|png|jpg|jpeg|svg|woff2?)$/)) {
    event.respondWith(
      caches.match(event.request).then(cachedResponse => {
        if (cachedResponse) {
          return cachedResponse;
        }

        return fetch(event.request).then(response => {
          // Cache successful responses
          if (response && response.status === 200) {
            const responseToCache = response.clone();
            caches.open(CACHE_NAME).then(cache => {
              cache.put(event.request, responseToCache);
            });
          }
          return response;
        });
      })
    );
    return;
  }

  // Network-first strategy for everything else (HTML, manifest, API calls)
  event.respondWith(
    fetch(event.request)
      .then(response => {
        // Cache successful responses
        if (response && response.status === 200) {
          const responseToCache = response.clone();
          caches.open(CACHE_NAME).then(cache => {
            cache.put(event.request, responseToCache);
          });
        }
        return response;
      })
      .catch(() => {
        // Fallback to cache on network failure
        return caches.match(event.request);
      })
  );
});
