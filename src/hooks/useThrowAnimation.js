import { useCallback, useEffect, useRef, useState } from "react";
import { animationConfig, BattleAnimationState } from "../config/animationConfig";
import { projectileIcons } from "../utils/assetLoader";
import { getRandomInteger, getRandomItem } from "../utils/random";

const initialAnimation = {
  phase: BattleAnimationState.IDLE,
  trainerFrame: 1,
  projectileIcon: null,
  projectileArcHeight: null,
  enemyIsHit: false,
  throwSpeed: 1,
};

export function useThrowAnimation() {
  const [animation, setAnimation] = useState(initialAnimation);
  const animationRef = useRef(initialAnimation);
  const mountedRef = useRef(true);
  const speedRef = useRef(1);
  const lastTapRef = useRef(null);
  const animationFrameRef = useRef(null);
  const pendingThrowRef = useRef(false);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      cancelAnimationFrame(animationFrameRef.current);
    };
  }, []);

  const updateAnimation = useCallback((next) => {
    animationRef.current = next;
    setAnimation(next);
  }, []);

  // Advances virtual animation time. A faster tap speed means each real frame
  // advances more virtual milliseconds, without overlapping attacks.
  const waitForAnimationTime = useCallback((duration) => new Promise((resolve) => {
    let elapsed = 0;
    let previousTime;

    const tick = (time) => {
      previousTime ??= time;
      elapsed += (time - previousTime) * speedRef.current;
      previousTime = time;

      if (elapsed >= duration) {
        resolve();
        return;
      }

      animationFrameRef.current = requestAnimationFrame(tick);
    };

    animationFrameRef.current = requestAnimationFrame(tick);
  }), []);

  const startThrow = useCallback(async () => {
    if (animationRef.current.phase !== BattleAnimationState.IDLE) return;

    updateAnimation({
      ...initialAnimation,
      phase: BattleAnimationState.THROWING,
      throwSpeed: speedRef.current,
    });

    for (const trainerFrame of [2, 3, 4]) {
      await waitForAnimationTime(animationConfig.trainerFrameDuration);
      if (!mountedRef.current) return;
      updateAnimation({ ...animationRef.current, trainerFrame });
    }

    await waitForAnimationTime(animationConfig.projectileLaunchDelay);
    if (!mountedRef.current) return;
    updateAnimation({
      ...animationRef.current,
      phase: BattleAnimationState.PROJECTILE_FLYING,
      projectileIcon: getRandomItem(projectileIcons),
      projectileArcHeight: getRandomInteger(
        animationConfig.projectileArcHeight.min,
        animationConfig.projectileArcHeight.max,
      ),
    });
  }, [updateAnimation, waitForAnimationTime]);

  const registerTap = useCallback(() => {
    const isHitAnimation = animationRef.current.phase === BattleAnimationState.HIT;
    const now = performance.now();
    const interval = lastTapRef.current === null ? null : now - lastTapRef.current;
    lastTapRef.current = now;

    const { slowInterval, fastInterval, maxMultiplier } = animationConfig.tapSpeed;
    const clampedInterval = Math.min(slowInterval, Math.max(fastInterval, interval ?? slowInterval));
    const intensity = (slowInterval - clampedInterval) / (slowInterval - fastInterval);
    const nextSpeed = 1 + intensity * (maxMultiplier - 1);
    speedRef.current = nextSpeed;

    if (isHitAnimation) {
      // Never interrupt or speed up hit feedback; remember one follow-up throw.
      pendingThrowRef.current = true;
      return;
    }

    updateAnimation({ ...animationRef.current, throwSpeed: nextSpeed });
    if (animationRef.current.phase === BattleAnimationState.IDLE) startThrow();
  }, [startThrow, updateAnimation]);

  const completeProjectile = useCallback(async () => {
    if (animationRef.current.phase !== BattleAnimationState.PROJECTILE_FLYING) return;

    updateAnimation({
      ...animationRef.current,
      phase: BattleAnimationState.HIT,
      projectileIcon: null,
      enemyIsHit: true,
    });

    // Hit feedback always plays at its authored duration, unaffected by tapping.
    await new Promise((resolve) => window.setTimeout(resolve, animationConfig.hitDuration));
    if (!mountedRef.current) return;

    const shouldThrowAgain = pendingThrowRef.current;
    pendingThrowRef.current = false;
    updateAnimation(initialAnimation);

    if (shouldThrowAgain) {
      startThrow();
    } else {
      speedRef.current = 1;
    }
  }, [startThrow, updateAnimation]);

  return {
    ...animation,
    isAnimating: animation.phase !== BattleAnimationState.IDLE,
    registerTap,
    completeProjectile,
  };
}