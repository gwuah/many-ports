#include <linux/bpf.h>

#include <bpf/bpf_endian.h>
#include <bpf/bpf_helpers.h>

struct bpf_map_def SEC("maps") ports = {
	.type		= BPF_MAP_TYPE_HASH,
	.max_entries	= 1024,
	.key_size	= sizeof(__u16),
	.value_size	= sizeof(__u8),
};

struct bpf_map_def SEC("maps") dedicated_socket = {
	.type		= BPF_MAP_TYPE_SOCKMAP,
	.max_entries	= 1,
	.key_size	= sizeof(__u32),
	.value_size	= sizeof(__u64),
};

SEC("sk_lookup/steer")
int steer(struct bpf_sk_lookup *ctx)
{
	const __u32 zero = 0;
	struct bpf_sock *sk;
	__u16 port;
	__u8 *open;
	long err;

	port = ctx->local_port;
	open = bpf_map_lookup_elem(&ports, &port);
	if (!open) {
		return SK_PASS;
	}

	sk = bpf_map_lookup_elem(&dedicated_socket, &zero);
	if (!sk) {
		return SK_DROP;
	}

	err = bpf_sk_assign(ctx, sk, 0);
	bpf_sk_release(sk);
	return err ? SK_DROP : SK_PASS;
}
SEC("license") const char __license[] = "Dual BSD/GPL";