@binding(0) @group(0) var tileTexture: texture_2d<f32>;
@binding(1) @group(0) var tileSampler: sampler;


struct Fragment {
    @builtin(position) Position : vec4<f32>,
    @location(0) TexCoord : vec2<f32>
};

// fullscreen triangle position and uvs
fn fullscreen_pos(i: u32) -> vec2<f32> {
  var p: vec2<f32>;
    
  // 3-vertex fullscreen triangle
  switch i {
    case 0u: { p = vec2<f32>(-1.0, -3.0); }
    case 1u: { p = vec2<f32>(3.0,  1.0); }
    default: { p = vec2<f32>(-1.0,  1.0); }
  }
    return p;
}


fn fullscreen_uv(i: u32) -> vec2<f32> {
    var p: vec2<f32>;

  // 3-vertex fullscreen triangle
  switch i {
    case 0u: { p = vec2<f32>(0.0, 2.0); }
    case 1u: { p = vec2<f32>(2.0,  0.0); }
    default: { p = vec2<f32>(0.0,  0.0); }
  }
    return p;
}


@vertex
fn vs_main (@builtin(vertex_index) VertexIndex : u32) -> Fragment  {

    var output : Fragment;

    output.Position = vec4<f32>(fullscreen_pos(VertexIndex), 0.0, 1.0);
    output.TexCoord = vec2<f32>(fullscreen_uv(VertexIndex));

    return output;
}


@fragment
fn fs_main (@location(0) TexCoord: vec2<f32>) -> @location(0) vec4<f32> {
    var col = textureSample(tileTexture, tileSampler, TexCoord);
    return vec4<f32>(col.rgb, 1.0);

    //return vec4<f32>(0.0, 1.0, 1.0, 0.5);
}
