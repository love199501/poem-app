// 极简全局声明，便于离线用 TS 编写页面而不报错。
// 生产项目建议安装 npm 包 miniprogram-api-typings 获得完整类型。
declare const wx: any;
declare const App: (opts: any) => void;
declare const Page: (opts: any) => void;
declare const Component: (opts: any) => void;
declare const getApp: () => any;
declare const getCurrentPages: () => any[];
declare const requirePlugin: (name: string) => any;

interface IAnyObject {
  [key: string]: any;
}
