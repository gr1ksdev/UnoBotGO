"""Generate review comparisons and an unaccelerated clip of the real local game."""
from pathlib import Path
import json
import subprocess
from PIL import Image, ImageDraw, ImageFont

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / '.reports/partida-pixi'
REF = ROOT / 'docs/design-reference/unobotgo-v2/references'
SCREENS = OUT / 'final-states'
font = ImageFont.truetype('/usr/share/fonts/TTF/DejaVuSans.ttf', 16)
comparisons = OUT / 'comparisons'
comparisons.mkdir(exist_ok=True)
for original, current, name in [
    ('original-meu-turno.jpg', 'game-390.png', 'minha-vez'),
    ('original-turno-adversario.jpg', 'other-turn-390.png', 'outra-vez'),
    ('original-coringa-cores.jpg', 'picker-390.png', 'seletor'),
    ('original-coringa-escolhido.jpg', 'picker-chosen-390.png', 'cor-escolhida'),
    ('original-resultado.jpg', 'result-390.png', 'resultado'),
    ('original-resultado-voto.jpg', 'result-vote-390.png', 'revanche'),
]:
    a = Image.open(REF / original).convert('RGB')
    b = Image.open(SCREENS / current).convert('RGB')
    def fit(image):
        return image.resize((round(image.width * 844 / image.height), 844), Image.Resampling.LANCZOS)
    a, b = fit(a), fit(b)
    canvas = Image.new('RGB', (a.width + b.width + 16, 882), '#eef2f3')
    draw = ImageDraw.Draw(canvas)
    draw.text((8, 8), 'Original fornecido', font=font, fill='#14262b')
    draw.text((a.width + 24, 8), 'Implementacao / fixture local', font=font, fill='#14262b')
    canvas.paste(a, (0, 38))
    canvas.paste(b, (a.width + 16, 38))
    canvas.save(comparisons / f'{name}.png')

production = OUT / 'production'
report = json.loads((production / 'browser.json').read_text())
segments = [next(x for x in report['video_segments'] if x['kind'] == kind and x['account'] == account) for kind, account in [('deal', 2), ('draw-and-flip', 1), ('color', 2)]]
filters = [f"[{c['account']-1}:v]trim=start={c['start']}:duration={c['seconds']},setpts=PTS-STARTPTS[v{i}]" for i,c in enumerate(segments)]
filters.append(''.join(f'[v{i}]' for i in range(len(segments))) + f'concat=n={len(segments)}:v=1:a=0,fps=25,format=yuv420p[out]')
video = production / 'video/movimentos-reais-11s.mp4'
subprocess.run(['ffmpeg','-y','-loglevel','error','-i',str(production / 'video/account-1.webm'),'-i',str(production / 'video/account-2.webm'),'-filter_complex',';'.join(filters),'-map','[out]','-c:v','libx264','-preset','fast','-crf','20','-movflags','+faststart',str(video)],check=True)
(production / 'video/segments.json').write_text(json.dumps({'sources':['account-1.webm','account-2.webm'],'real_backend':True,'telegram_real':False,'segments':segments},indent=2))
subprocess.run(['ffmpeg','-y','-loglevel','error','-i',str(video),'-vf','fps=2,scale=195:422,tile=7x3','-frames:v','1',str(production / 'video/contato-movimentos.png')],check=True)
