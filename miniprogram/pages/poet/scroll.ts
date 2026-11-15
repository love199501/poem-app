import { request } from '../../utils/request';
import { ensureLogin } from '../../utils/auth';
import { ensureAskCredit } from '../../utils/askAd';

function poetAskPrompts(stages: any[]) {
  const aged = (stages || []).filter((s) => Number(s.age) > 0);
  const early = aged[0];
  const late = aged.length > 1 ? aged[aged.length - 1] : null;
  const list = [
    early ? '你' + early.label + '前后，日子是怎么过的？' : '你年轻的时候，最放不下的是什么？',
    '你这一生里，哪一段最难熬？',
    late ? '到了' + late.label + '，你怎么看自己走过的路？' : '如果回头看这一生，你最想对自己说什么？',
  ];
  const out: string[] = [];
  list.forEach((s) => {
    if (out.length < 3 && s.indexOf('你') >= 0 && /[？?]$/.test(s) && out.indexOf(s) < 0) out.push(s);
  });
  return out;
}

Page({
  data: {
    author: '',
    dynasty: '',
    count: 0,
    stages: [] as any[],
    intro: '',
    introLoading: false,
    loading: true,
    authorChar: '诗',
    isMember: false,
    askOpen: false,
    askInput: '',
    askBusy: false,
    asks: [] as any[],
    askPrompts: [] as string[],
    askRemain: 0,
    askPerAd: 3,
  },

  onLoad(options: any) {
    const author = decodeURIComponent(options.author || '');
    const dynasty = decodeURIComponent(options.dynasty || '');
    this.setData({
      author,
      dynasty,
      authorChar: String(author || '诗').slice(0, 1),
      askPrompts: poetAskPrompts([]),
    });
    if (author) wx.setNavigationBarTitle({ title: author });
    this.load(author, dynasty);
  },

  onShow() {
    this.refreshMember();
    this.loadAskCredit();
  },

  loadAskCredit() {
    if (!this.data.author) return;
    ensureLogin().then((uid) => {
      if (!uid) return;
      request<any>({
        url: '/users/me/ask-credit',
        data: { author: this.data.author, dynasty: this.data.dynasty || '' },
        silent: true,
      })
        .then((r) => {
          if (!r) return;
          const patch: any = { askRemain: r.remain, askPerAd: r.perAd || 3 };
          if (r.remain < 0) patch.isMember = true;
          this.setData(patch);
        })
        .catch(() => {});
    });
  },

  refreshMember() {
    ensureLogin().then((uid) => {
      if (!uid) return;
      request<any>({ url: '/users/me', silent: true }).then((res) => {
        const u = (res && res.user) || res || {};
        const app = getApp();
        if (app && app.globalData) app.globalData.user = u;
        this.setData({ isMember: !!u.isMember });
      }).catch(() => {});
    });
  },

  load(author: string, dynasty: string) {
    if (!author) {
      this.setData({ loading: false });
      return;
    }
    request<any>({ url: '/poets/scroll', data: { author, dynasty } })
      .then((res) => {
        const intro = (res && res.intro) || '';
        const stages = res.stages || [];
        this.setData({
          dynasty: res.dynasty || '',
          count: res.count || 0,
          stages,
          intro,
          introLoading: !intro,
          loading: false,
          askPrompts: poetAskPrompts(stages),
        });
        if (!intro) this.loadIntro(author, dynasty || res.dynasty || '');
      })
      .catch(() => this.setData({ loading: false }));
  },

  loadIntro(author: string, dynasty: string) {
    ensureLogin().then((uid) => {
      if (!uid) {
        this.setData({ introLoading: false });
        return;
      }
      request<any>({ url: '/poets/intro', data: { author, dynasty }, silent: true })
      .then((res) => {
        const intro = (res && res.intro) || '';
        this.setData({ intro, introLoading: false });
      })
      .catch(() => this.setData({ introLoading: false }));
    });
  },

  goDetail(e: any) {
    const id = e.currentTarget.dataset.id;
    if (!id) return;
    wx.navigateTo({ url: '/pages/poem/detail?id=' + id });
  },

  toggleAsk() {
    if (this.data.askOpen) {
      this.setData({ askOpen: false });
      return;
    }
    ensureAskCredit({
      isMember: this.data.isMember,
      remain: this.data.askRemain,
      author: this.data.author,
      dynasty: this.data.dynasty,
      forceAd: true,
    }).then((remain) => {
      if (remain == null) return;
      this.setData({ askRemain: remain, askOpen: true });
    });
  },

  onAskInput(e: any) {
    this.setData({ askInput: e.detail.value });
  },

  sendAsk(e?: any) {
    const fromChip = e && e.currentTarget && e.currentTarget.dataset ? e.currentTarget.dataset.q : '';
    const q = String(fromChip || this.data.askInput || '').trim();
    if (!q || this.data.askBusy) return;
    ensureAskCredit({
      isMember: this.data.isMember,
      remain: this.data.askRemain,
      author: this.data.author,
      dynasty: this.data.dynasty,
    }).then((remain) => {
      if (remain == null) return;
      this.setData({ askRemain: remain });
      this.postAsk(q);
    });
  },

  postAsk(q: string) {
    const history = (this.data.asks || [])
      .filter((m: any) => m.role === 'user' || m.role === 'ai')
      .reduce((acc: any[], m: any, i: number, arr: any[]) => {
        if (m.role === 'user') {
          const next = arr[i + 1];
          acc.push({ question: m.content, answer: next && next.role === 'ai' ? next.content : '' });
        }
        return acc;
      }, [])
      .filter((h: any) => h.answer);
    const asks = (this.data.asks || []).concat([{ role: 'user', content: q }]);
    this.setData({ asks, askInput: '', askBusy: true, askOpen: true });
    ensureLogin().then((uid) => {
      if (!uid) {
        this.setData({ askBusy: false });
        return;
      }
      request<any>({
        url: '/poets/ask',
        method: 'POST',
        data: {
          author: this.data.author,
          dynasty: this.data.dynasty,
          question: q,
          history,
        },
        silent: true,
      })
        .then((r) => {
          const patch: any = {
            askBusy: false,
            asks: (this.data.asks || []).concat([{ role: 'ai', content: r.answer || '', source: r.source || '' }]),
          };
          if (r && typeof r.askRemain === 'number') patch.askRemain = r.askRemain;
          this.setData(patch);
        })
        .catch((err: any) => {
          const asks = (this.data.asks || []).slice();
          if (asks.length && asks[asks.length - 1].role === 'user') asks.pop();
          const needAd = !!(err && String(err.msg || '').indexOf('广告') >= 0);
          this.setData({
            askBusy: false,
            asks,
            askInput: q,
            askRemain: needAd ? 0 : this.data.askRemain,
          });
          wx.showToast({ title: (err && err.msg) || '暂时无法回答', icon: 'none' });
        });
    });
  },
});
