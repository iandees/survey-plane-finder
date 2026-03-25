const Colors = (() => {
  const palette = [
    '#e84a4a', '#e8a44a', '#4a9de8', '#9b59b6', '#2ecc71',
    '#e74c3c', '#f39c12', '#1abc9c', '#3498db', '#8e44ad',
    '#d35400', '#27ae60', '#2980b9', '#c0392b', '#16a085',
  ];

  function forHex(hex) {
    let hash = 0;
    for (let i = 0; i < hex.length; i++) {
      hash = (hash * 31 + hex.charCodeAt(i)) | 0;
    }
    return palette[Math.abs(hash) % palette.length];
  }

  return { forHex, palette };
})();
