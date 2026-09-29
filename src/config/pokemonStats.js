// 宝可梦基础属性：生命值与攻击力
// 敌方更强，我方较弱 —— 需要训练师扔球助攻才能取胜
export const pokemonStats = {
  player: {
    name: "Eevee",
    maxHp: 140,
    attack: 7,
    throwBonus: 20, // 训练师每扔一次球造成的额外伤害
  },
  enemy: {
    name: "Bulbasaur",
    maxHp: 320,
    attack: 18,
  },
};

// 自动攻击节奏（毫秒）
export const battleConfig = {
  autoAttackInterval: 2500,
  attackRetryDelay: 180,
  attackDashDuration: 280,   // 冲到对手身上的时间
  attackReturnDuration: 280, // 碰撞后弹回的时间
  hitFlashDuration: 260,
};

// 根据剩余生命值比例返回血条颜色
export function getHealthColor(ratio) {
  if (ratio > 0.5) return "#4cd964";
  if (ratio > 0.2) return "#ffcc00";
  return "#ff3b30";
}
