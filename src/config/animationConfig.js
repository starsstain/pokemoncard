export const animationConfig = {
  trainerFrameDuration: 120,
  projectileLaunchDelay: 70,
  projectileDuration: 620,
  projectileArcHeight: { min: 34, max: 72 },
  hitDuration: 260,
  tapSpeed: {
    slowInterval: 420,
    fastInterval: 50,
    maxMultiplier: 15.2,
  },
};

export const BattleAnimationState = Object.freeze({
  IDLE: "idle",
  THROWING: "throwing",
  PROJECTILE_FLYING: "projectileFlying",
  HIT: "hit",
});