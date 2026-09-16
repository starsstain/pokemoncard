// Coordinates use the original 320 x 240 GBA battle frame.
// Keeping them here makes it easy to fine-tune a scene without touching components.
export const sceneSize = { width: 320, height: 240 };

export const battleLayout = {
  enemyPlatform: { x: 184, y: 58, width: 128, height: 64 },
  playerPlatform: { x: 8, y: 163, width: 128, height: 64 },
  // Shifted 33 scene pixels left: about 100px at the 960px desktop display size.
  playerTrainer: { x: 2, y: 137, width: 64, height: 64 },
  playerPokemon: { x: 60, y: 140, width: 64, height: 64 },
  enemyPokemon: { x: 219, y: 38, width: 64, height: 64 },
  projectile: {
    start: { x: 47, y: 161 },
    target: { x: 251, y: 70 },
  },
};