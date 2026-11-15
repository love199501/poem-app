// 后端访问配置：支持两种部署方式
//  1) 微信云托管「原生调用」(推荐)：填 CLOUD_ENV + CLOUD_SERVICE，走 wx.cloud.callContainer，
//     免配 request 合法域名，且自动带上用户 openid（后端免鉴权登录）。
//  2) 普通 HTTPS：把后端用「公网访问」暴露出来，填 BASE_URL 为该 https 域名，
//     并在小程序后台把域名加入 request/uploadFile 合法域名（开发者工具可勾「不校验合法域名」）。

// 填了 CLOUD_ENV 即走云托管原生调用；留空则用 BASE_URL。
export const CLOUD_ENV = 'prod-d1g9dvdg5f5f52c67'; // 云托管环境 ID，如 prod-xxxxxx
export const CLOUD_SERVICE = 'golang-iyjf'; // 云托管服务名

// 普通模式后端地址（本地 localhost；云上公网访问则填对应 https 域名）
export const BASE_URL = 'https://golang-iyjf-344746-6-1528738627.sh.run.tcloudbase.com';
export const API = BASE_URL + '/api';

// 激励视频广告位。留空则从后端 /app/config 的 rewardAdUnit 读取（环境变量 REWARD_AD_UNIT）。
export const REWARD_AD_UNIT = '';

export function useCloud(): boolean {
  return !!CLOUD_ENV;
}
