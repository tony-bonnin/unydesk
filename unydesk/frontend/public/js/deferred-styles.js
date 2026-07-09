(() => {
  const links = document.querySelectorAll('link[rel="preload"][as="style"][data-deferred-stylesheet]');

  for (const link of links) {
    link.rel = 'stylesheet';
    link.removeAttribute('as');
    link.removeAttribute('data-deferred-stylesheet');
  }
})();
