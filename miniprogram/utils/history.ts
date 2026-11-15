const KEY = 'read_poem_ids';

// localReadIds 本地记下的已读诗 id，用来在服务端记录返回前就避开这些卡。
export function localReadIds(): number[] {
  const raw = wx.getStorageSync(KEY);
  if (!Array.isArray(raw)) return [];
  const out: number[] = [];
  raw.forEach((n: any) => {
    const id = Number(n);
    if (id > 0 && out.indexOf(id) < 0) out.push(id);
  });
  return out;
}

export function rememberRead(poemId: number) {
  const id = Number(poemId || 0);
  if (!id) return;
  const ids = localReadIds();
  if (ids.indexOf(id) >= 0) return;
  ids.push(id);
  wx.setStorageSync(KEY, ids);
}

export function saveReadIds(ids: number[]) {
  wx.setStorageSync(KEY, ids);
}
