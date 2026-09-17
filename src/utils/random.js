export function getRandomItem(items) {
  if (!items.length) return null;
  return items[Math.floor(Math.random() * items.length)];
}

export function getRandomInteger(min, max) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}