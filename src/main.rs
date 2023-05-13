use std::fs;
use std::path::Path;

use kube::Client;

use crate::k8s_discovery::{DiscoveryNamespaces, K8sDiscovery};

mod k8s_discovery;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let client = Client::try_default().await?;

    let mut discovery = K8sDiscovery::new(
        client,
        DiscoveryNamespaces::All,
        true,
        vec!(
            "event".to_string(),
            "componentstatus".to_string(),
            "v1/endpoints".to_string(),
            "cilium.io/ciliumendpoint".to_string(),
            "discovery.k8s.io/endpointslice".to_string(),
            "cilium.io/ciliumnode".to_string(),
            "apps/replicaset".to_string(),
        ),
        "{namespace}/{kind_l}/{name}.yaml".to_string(),
    );

    let (se, mut rx) = K8sDiscovery::channel();
    tokio::spawn({
        let d = discovery.clone();
        async move { d.clone().discover(se).await }
    });

    while let Some((item, res)) = rx.recv().await {
        let file_name = discovery.format_file_name(&item, &res);
        let file_path = Path::new("./data").join(file_name).to_str().unwrap().to_string();
        let sanitized_item = discovery.sanitize_item(&item, &res);
        let data = discovery.serialize_item(&sanitized_item, &res);

        fs::create_dir_all(Path::new(file_path.as_str()).parent().unwrap().as_os_str())?;
        fs::write(file_path.as_str(), data)?;
    }

    Ok(())
}
