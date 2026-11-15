import { request } from '../../utils/request';
import { loadCommentEnabled } from '../../utils/flags';

const PAGE = 24;

function hasText(p: any) {
  return !!String((p && p.firstVerse) || '').trim();
}

Page({
  data: {
    keyword: '',
    dynasty: '全部',
    mood: '全部',
    author: '全部',
    tag: '全部',
    // 筛选项来自后端实时聚合（库中实际涉及的维度），默认仅「全部」占位。
    dynasties: ['全部'] as string[],
    moods: ['全部'] as string[],
    authors: ['全部'] as string[],
    tags: ['全部'] as string[],
    poems: [] as any[],
    shown: 0,
    total: 0,
    loading: false,
    commentEnabled: false,
  },

  // 每次进入/切回「诗库」tab：先用已有诗单画出第一屏，再在后台刷新。
  onShow() {
    loadCommentEnabled().then((on) => this.setData({ commentEnabled: on }));
    this.fetchFacets();
    const mood = wx.getStorageSync('lib_mood');
    if (mood) {
      wx.removeStorageSync('lib_mood');
      this.setData({ mood }, () => {
        if (this._all) this.render(true);
      });
    } else if (this._all) {
      this.render(false);
    }
    this.loadCatalog();
  },

  // 拉取库中实际涉及的朝代 / 心情 / 诗人 / 标签作为筛选项。
  fetchFacets() {
    request<any>({ url: '/poems/facets' })
      .then((f) => {
        this.setData({
          dynasties: f.dynasties || ['全部'],
          moods: f.moods || ['全部'],
          authors: f.authors || ['全部'],
          tags: f.tags || ['全部'],
        });
      })
      .catch(() => {});
  },

  // 全库只拉一次。筛选在本地完成，点朝代或心情不用再等接口。
  loadCatalog() {
    const seq = (this._seq = (this._seq || 0) + 1);
    if (!this._all) this.setData({ loading: true });
    request<any[]>({ url: '/poems' })
      .then((list) => {
        if (seq !== this._seq) return;
        this._all = (list || []).filter(hasText);
        this.render(!this.data.poems.length);
      })
      .catch(() => {
        if (seq === this._seq) this.setData({ loading: false });
      });
  },

  match(p: any) {
    const d = this.data;
    if (d.dynasty !== '全部' && p.dynasty !== d.dynasty) return false;
    if (d.author !== '全部' && p.author !== d.author) return false;
    if (d.mood !== '全部' && (p.moods || []).indexOf(d.mood) < 0) return false;
    if (d.tag !== '全部' && (p.tags || []).indexOf(d.tag) < 0) return false;
    const q = String(d.keyword || '').trim();
    if (q) {
      const blob = (p.title || '') + (p.author || '') + (p.firstVerse || '') + (p.dynasty || '');
      if (blob.indexOf(q) < 0) return false;
    }
    return true;
  },

  // 先挂第一屏。滑到底再续上，避免一次把几百首画进页面。
  render(reset: boolean) {
    const list = (this._all || []).filter((p: any) => this.match(p));
    this._view = list;
    let n = reset ? PAGE : this.data.shown || PAGE;
    if (n < PAGE) n = PAGE;
    if (n > list.length) n = list.length;
    this.setData({
      poems: list.slice(0, n),
      shown: n,
      total: list.length,
      loading: false,
    });
  },

  onReachBottom() {
    const list = this._view || [];
    const n = this.data.shown || 0;
    if (n >= list.length) return;
    const next = Math.min(list.length, n + PAGE);
    this.setData({ poems: list.slice(0, next), shown: next });
  },

  onInput(e: any) {
    this.setData({ keyword: e.detail.value });
  },
  onSearch() {
    this.render(true);
  },
  pickDynasty(e: any) {
    this.setData({ dynasty: e.currentTarget.dataset.v }, () => this.render(true));
  },
  pickMood(e: any) {
    this.setData({ mood: e.currentTarget.dataset.v }, () => this.render(true));
  },
  pickAuthor(e: any) {
    this.setData({ author: e.currentTarget.dataset.v }, () => this.render(true));
  },
  pickTag(e: any) {
    this.setData({ tag: e.currentTarget.dataset.v }, () => this.render(true));
  },

  goDetail(e: any) {
    const id = e.currentTarget.dataset.id;
    wx.navigateTo({ url: '/pages/poem/detail?id=' + id });
  },
});
