import { request } from '../../utils/request';
import { BASE_URL } from '../../config';

const TITLES: Record<string, string> = {
  favorites: '我的收藏',
  comments: '我的评论',
  likes: '获赞评论',
};

function toFull(u: string) {
  if (!u) return '';
  return /^https?:/.test(u) ? u : BASE_URL + u;
}
function fmtDate(s: string) {
  return (s || '').slice(0, 10);
}

Page({
  data: {
    type: 'favorites',
    title: '',
    poems: [] as any[],
    comments: [] as any[],
    loading: true,
    empty: false,
  },

  onLoad(options: any) {
    const type = options.type || 'favorites';
    const title = TITLES[type] || '列表';
    wx.setNavigationBarTitle({ title });
    this.setData({ type, title });
    this.fetch();
  },

  fetch() {
    const type = this.data.type;
    if (type === 'favorites') {
      request<any[]>({ url: '/users/me/favorites' })
        .then((list) => {
          const poems = (list || []).map((p) => ({ ...p, illust: toFull(p.illustrationUrl || '') }));
          this.setData({ poems, loading: false, empty: poems.length === 0 });
        })
        .catch(() => this.setData({ loading: false, empty: true }));
    } else {
      // 评论与被赞均按时间倒排（最近在最上）。
      request<any[]>({ url: '/users/me/comments', data: { sort: 'new' } })
        .then((list) => {
          const comments = (list || []).map((c) => ({ ...c, dateText: fmtDate(c.createdAt) }));
          this.setData({ comments, loading: false, empty: comments.length === 0 });
        })
        .catch(() => this.setData({ loading: false, empty: true }));
    }
  },

  goDetail(e: any) {
    const id = e.currentTarget.dataset.id;
    if (id) wx.navigateTo({ url: '/pages/poem/detail?id=' + id });
  },
});
