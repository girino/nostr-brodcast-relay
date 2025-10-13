# Midjourney Prompts for Broadcast Relay Visuals

## Icon Prompts

### Option 1: Modern Tech Icon
```
/imagine a minimalist logo icon for a Nostr broadcast relay service, purple gradient (#667eea to #764ba2), featuring interconnected broadcast waves radiating from a central node, modern tech aesthetic, clean geometric shapes, circular frame, white background, vector style, professional, 1024x1024 --v 6 --style raw
```

### Option 2: Signal Broadcast Icon  
```
/imagine a sleek relay icon showing signal waves broadcasting in all directions, purple and blue color scheme (#667eea, #2196F3), minimalist geometric design, circular badge format, tech startup aesthetic, gradient mesh, white background, 1024x1024 --v 6 --style raw
```

### Option 3: Network Hub Icon
```
/imagine a modern network hub icon with nodes and connections, representing a Nostr relay broadcaster, purple gradient theme (#667eea to #764ba2), isometric perspective, glowing connections, circular frame, white background, clean vector art, 1024x1024 --v 6 --style raw
```

### Option 4: Satellite Dish Tech
```
/imagine a stylized satellite dish icon broadcasting data streams, purple and violet gradients (#667eea, #764ba2), modern minimalist design, circular logo format, tech aesthetic, geometric shapes, white background, vector style, 1024x1024 --v 6 --style raw
```

## Banner Prompts

### Option 1: Abstract Tech Network
```
/imagine a wide banner image showing abstract network visualization with flowing data streams, purple and blue gradients (#667eea to #764ba2), interconnected nodes, digital waves, modern tech aesthetic, clean and professional, wide format 1500x500, minimalist design --v 6 --style raw
```

### Option 2: Broadcast Waves
```
/imagine a banner showing concentric broadcast waves emanating from center, purple gradient background (#667eea to #764ba2), glowing particles, ethereal atmosphere, modern tech aesthetic, wide format 1500x500, professional and clean --v 6 --style raw
```

### Option 3: Data Highway
```
/imagine an abstract data highway visualization banner, flowing information streams, purple and blue color scheme (#667eea, #2196F3, #764ba2), modern tech design, geometric patterns, particle effects, wide format 1500x500, professional aesthetic --v 6 --style raw
```

### Option 4: Network Mesh
```
/imagine a futuristic network mesh visualization for a banner, interconnected glowing nodes, purple gradient theme (#667eea to #764ba2), flowing data particles, abstract geometric patterns, modern and clean, wide format 1500x500 --v 6 --style raw
```

### Option 5: Cyberpunk Minimal
```
/imagine a minimalist cyberpunk-inspired banner with neon purple accents (#667eea, #764ba2), abstract geometric shapes, digital grid, flowing light trails, modern tech aesthetic, professional and clean, wide format 1500x500 --v 6 --style raw
```

## Recommended Specifications

### Icon
- **Size**: 1024x1024px (square)
- **Format**: PNG with transparency, or JPEG
- **Style**: Minimalist, modern, professional
- **Colors**: Purple gradient (#667eea to #764ba2) with white/blue accents
- **Usage**: Appears in relay info, NIP-11, and main page

### Banner
- **Size**: 1500x500px (3:1 ratio)
- **Format**: JPEG or PNG
- **Style**: Modern, abstract, tech-inspired
- **Colors**: Purple gradient theme matching icon
- **Usage**: Top of main page, creates professional impression

## Post-Processing Tips

1. **Icon**: 
   - Ensure good contrast on both light and dark backgrounds
   - Add subtle glow or shadow for depth
   - Keep it simple - should be recognizable even at 32x32px

2. **Banner**:
   - Test at different screen sizes (mobile, desktop)
   - Ensure text remains readable if overlaid
   - Use gradient overlay for better text contrast
   - Keep important elements center-focused

## Alternative: Use DALL-E or Stable Diffusion

These prompts can be adapted for other AI image generators:
- Remove `--v 6 --style raw` flags
- Keep the core description
- Adjust color codes as hex values
- Emphasize "professional" and "clean" for business use

## Where to Host

After generating images:
- Upload to IPFS (decentralized, permanent)
- Use imgbb.com or similar (free hosting)
- Host on your own server
- Use GitHub Pages or CDN

Update `.env` file with URLs:
```
RELAY_ICON=https://your-domain.com/icon.png
RELAY_BANNER=https://your-domain.com/banner.jpg
```

