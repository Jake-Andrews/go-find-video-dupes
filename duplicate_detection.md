https://github.com/facebook/ThreatExchange/
https://www.hackerfactor.com/blog/index.php?/archives/971-FB-TMK-PDQ-WTF.html
> Before using `ffmpeg-go`, FFmpeg must be installed and accessible via your
> path
https://www.cs.toronto.edu/~norouzi/research/papers/multi_index_hashing.pdf
https://norouzi.github.io/research/posters/mih_poster.pdf
Set Cpu limit/request For FFmpeg-go

e := ComplexFilterExample("./sample_data/in1.mp4", "./sample_data/overlay.png", "./sample_data/out2.mp4")
err := e.RunWithResource(0.1, 0.5)
if err != nil {
    assert.Nil(t, err)
}

TMK + PDQF
  - 256 bit hashes + 0-100 metric for detail level (blurry)
  - Skips binary-quantization step of PDQ (f floating point)
  - TMK collect timewise info about each frame
  - Final hashes are 265KB but the first 1KB differentiates almost all videos?

Syntactic vs semantic hashers
Semantic detect features in the images

TMK+PDQF runs at 30x multiple of video playback speeds
Syntactic hashers are good at finding media shared with little intentional
minipulation (reencoding, small artfiacts, etc...)
Cropping, large logos/watermarks, heavy use of filters,
etc...all break the algorithm

TMK algorithm
  - standardize videos to 15fps
  - PDQF produces 256 element vector for each frame
  - level-1 feature (256 element vector) calculated by averaging each frames vector over time element-wise, finalVector[2] = frame1[2] + frame2[2]...framen[2] / n
  - level-2 feature is weighted using cos/sin and different periods
  - if level-1 pair < threshold (0.7), not a match, don't calculate
    level-2 pair score
  - level-1 pair score suited to indexing systems FAISS, etc...for
    faster comparison

PDQ
  - P (perceptual hashing), D (spectral hashing, Discrete cosing
    transformation DCT), Q (quality metric, blurry, black, etc...)
  - 1. RGB to luminance
  - 2. two-pass jarosz filters, weighted average of 64x64 subblocks of thje
       luminance image. (recomends using off-the-sheft to resize to
       512x512 before RGB to luminance, since it's too time consuming
       otherwise
  - 3. with the 64x64, sum of absolute values of horizontal and vertical
       gradients (L1 norm of quantized gradients)., rescale this number
       so images with features have a score close to 100 and featureless
       images have a score of 0 (only one colour, etc..., no gradients).

Dihedral-transformation hashes
For dihedral group d4 with 8 elements:
identity, 90 degrees, 180, 270, horizontal reflection, vertical, diagonal #1,
diagonal #2 (across both diagonals of a square)

I-Frames
