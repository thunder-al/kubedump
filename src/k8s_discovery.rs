use std::path::Path;

use kube::{
    api::{Api, DynamicObject},
    Client,
    discovery::{Discovery, Scope, verbs},
};
use kube::api::ApiResource;
use kube::discovery::ApiGroup;
use rand::distributions::{Alphanumeric, DistString};
use tokio::sync::mpsc;
use tokio::sync::mpsc::{Receiver, Sender};

pub type DiscoveryItem = (DynamicObject, ApiResource);

#[derive(Clone)]
pub struct K8sDiscovery {
    namespaces: DiscoveryNamespaces,
    include_global_resources: bool,
    ignore_resources: Vec<String>,
    formatter: String,

    client: Client,
}

impl K8sDiscovery {
    pub fn new(
        client: Client,
        namespaces: DiscoveryNamespaces,
        include_global_resources: bool,
        ignore_resources: Vec<String>,
        formatter: String,
    ) -> K8sDiscovery {
        Self {
            namespaces,
            include_global_resources,
            ignore_resources,
            client,
            formatter,
        }
    }

    pub fn channel() -> (Sender<DiscoveryItem>, Receiver<DiscoveryItem>) {
        mpsc::channel(1)
    }

    pub async fn discover(&mut self, sender: Sender<DiscoveryItem>) {
        let discovery = Discovery::new(self.client.clone()).run().await.unwrap();

        for group in discovery.groups() {
            for (ar, caps) in group.recommended_resources() {
                if !caps.supports_operation(verbs::LIST) {
                    continue;
                }

                if caps.scope == Scope::Cluster {
                    if self.include_global_resources {
                        let api = Api::all_with(self.client.clone(), &ar);
                        let list = api.list(&Default::default()).await.unwrap();
                        self.dispatch_found_items(&sender, list.items, &ar, group).await.unwrap();
                    }
                } else {
                    let mut items: Vec<DynamicObject> = vec!();
                    if let DiscoveryNamespaces::Some(namespaces) = &self.namespaces {
                        for namespace in namespaces {
                            let api = Api::namespaced_with(self.client.clone(), namespace.as_str(), &ar);
                            items.append(&mut api.list(&Default::default()).await.unwrap().items);
                        }

                        self.dispatch_found_items(&sender, items, &ar, group).await.unwrap();
                    } else {
                        let api = Api::namespaced_with(self.client.clone(), "", &ar);
                        let list = api.list(&Default::default()).await.unwrap();
                        self.dispatch_found_items(&sender, list.items, &ar, group).await.unwrap();
                    }
                }
            }
        }
    }

    async fn dispatch_found_items(
        &mut self,
        sender: &Sender<DiscoveryItem>,
        list: Vec<DynamicObject>,
        ar: &ApiResource,
        group: &ApiGroup,
    ) -> Result<(), mpsc::error::SendError<DiscoveryItem>> {
        for item in list {
            if !self.ignore_resources.contains(&format!("{}/{}", group.name(), ar.kind)) {
                if {
                    !self.ignore_resources.contains(&format!("{}/{}", ar.api_version, ar.kind.to_lowercase()))
                        && !self.ignore_resources.contains(&format!("{}/{}", ar.group, ar.kind.to_lowercase()))
                        && !self.ignore_resources.contains(&format!("{}", ar.kind.to_lowercase()))
                } {
                    sender.send((item, ar.clone())).await?;
                }
            }
        }

        Ok(())
    }

    pub fn format_file_name(&mut self, item: &DynamicObject, res: &ApiResource) -> String {
        let mut name = self.formatter.clone();

        name = name.replace("{kind}", res.kind.as_str());
        name = name.replace("{kind_l}", res.kind.to_lowercase().as_str());
        name = name.replace("{v}", res.version.as_str());
        name = name.replace("{group}", res.group.as_str());
        name = name.replace("{namespace}", item.clone().metadata.namespace.unwrap_or("_cluster".to_string()).as_str());
        name = name.replace("{name}", item.clone().metadata.name.unwrap_or(item.clone().metadata.uid.unwrap_or(Alphanumeric.sample_string(&mut rand::thread_rng(), 16))).as_str());

        let dummy_file_name = "_.yaml";
        let file_name: String = Path::new(name.as_str())
            .file_name()
            .unwrap_or(Path::new(dummy_file_name).as_os_str())
            .to_str()
            .unwrap_or(dummy_file_name)
            .chars()
            .map(|x| match x {
                '0'..='9' => x,
                'A'..='Z' => x,
                'a'..='z' => x,
                '_' | '-' | '+' | '.' => x,
                _ => '_'
            })
            .collect();

        name = Path::new(name.as_str())
            .with_file_name(file_name)
            .to_str()
            .unwrap_or(dummy_file_name)
            .to_string();

        name
    }

    #[allow(unused_variables)]
    pub fn sanitize_item(&mut self, item: &DynamicObject, res: &ApiResource) -> DynamicObject {
        let mut item = item.clone();
        item.metadata.managed_fields = None;

        item
    }

    #[allow(unused_variables)]
    pub fn serialize_item(&mut self, item: &DynamicObject, res: &ApiResource) -> String {
        serde_yaml::to_string(&item).unwrap()
    }
}

#[derive(Clone)]
#[allow(dead_code)]
pub enum DiscoveryNamespaces {
    All,
    Some(Vec<String>),
}
