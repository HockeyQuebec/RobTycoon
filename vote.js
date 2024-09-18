
const axios = require('axios');
const { JSDOM } = require('jsdom');
const { to } = require('await-to-js');
const progress = require('cli-progress');
const randomUserAgent = require('random-useragent');
const Bottleneck = require('bottleneck');

const POLL = 14362395;
const OPTION = 63882947;

async function vote(total = 10) {
  console.log(`Voting ${total} times...`);
  // Wait 6 secs in between votes because of Poll Daddy's rate limits.
  // @see {@link https://bit.ly/2W0xzsN}
  const limiter = new Bottleneck({ maxConcurrent: 1, minTime: 6000 });
  let count = 0;
  const bar = new progress.SingleBar({}, progress.Presets.shades_classic);
  bar.start(total, count);
  limiter.on('done', () => bar.update((count += 1)));
  limiter.on('failed', async (error, jobInfo) => {
    console.error(`Job (${jobInfo.options.id}) failed:`, error);
    if (jobInfo.retryCount < 10) {
      console.log(`Retrying job (${jobInfo.options.id}) in 1 sec...`);
      return 1000;
    }
  });
  limiter.on('retry', (error, jobInfo) => {
    console.log(`Now retrying job (${jobInfo.options.id})...`);
  });
  await Promise.all(
    Array(total).fill(null).map((_, id) =>
      limiter.schedule({ id }, async () => {
        const headers = {
          'Accept': '*/*',
          'Accept-Language': 'en-US,en;q=0.5',
          'Cache-Control': 'no-cache',
          'Connection': 'keep-alive',
          'DNT': 1,
          'Host': 'poll.fm',
          'Pragma': 'no-cache',
          'Referer': `https://poll.fm/${POLL}`,
          'Sec-Fetch-Dest': 'empty',
          'Sec-Fetch-Mode': 'cors',
          'Sec-Fetch-Site': 'same-origin',
          'TE': 'trailers',
          'User-Agent': 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:91.0) Gecko/20100101 Firefox/91.0',
        };
        const res = await axios.get(`https://poll.fm/${POLL}`, { headers });
        headers['Cookie'] = res.headers['set-cookie'][0].split('; ')[0];
        const doc = new JSDOM(res.data).window.document;
        const btn = doc.querySelector('.vote-button');
        const data = JSON.parse(btn.dataset.vote);
        const pz = doc.querySelector('input[name="pz"]').value || 1;
        const url =
          `https://poll.fm/vote?va=${data.at}&pt=${data.m}&r=${data.b}` +
          `&p=${data.id}&a=${OPTION}&o=&t=${data.t}&token=${data.n}&pz=${pz}`;
        await axios.get(url, { headers });
      })
    )
  );
  bar.stop();
  console.log(`Voted ${count}/${total} times.`);
}

async function main(totalVotes = 50, repeatInterval = 1000) {
  while (true) {
    await vote(totalVotes);
    console.log(`Waiting ${repeatInterval / 1000} seconds before repeating...`);
    await new Promise(resolve => setTimeout(resolve, repeatInterval));
  }
}

main(25, 60000); // 50 votes per cycle, waits 1 minute before repeating